package portforward

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const socatImage = "alpine/socat"

type Tunnel struct {
	// direct mode (ssh -N -L): persistent ssh process
	cmd    *exec.Cmd
	stderr *bytes.Buffer

	// netns mode (socat via docker run): local listener + one ssh per connection
	listener net.Listener

	done chan struct{}

	mu       sync.Mutex
	closed   bool
	connCmds map[*exec.Cmd]struct{}
}

// NewTunnel forwards localPort to the target container.
// If hostPort > 0, the container publishes a port on the remote host and a
// plain ssh -L tunnel to 127.0.0.1:hostPort is used.
// Otherwise (overlay networks, unpublished ports), each connection is piped
// through a socat process running inside the container's network namespace.
func NewTunnel(sshHost string, localPort uint16, containerID string, containerPort, hostPort uint16) (*Tunnel, error) {
	if hostPort > 0 {
		return newDirectTunnel(sshHost, localPort, hostPort)
	}
	return newNetnsTunnel(sshHost, localPort, containerID, containerPort)
}

func newDirectTunnel(sshHost string, localPort, hostPort uint16) (*Tunnel, error) {
	user, addr := parseSSHHost(sshHost)
	host, port := splitHostPort(addr)

	// Fail fast if another process (e.g. a stale tunnel) already owns the port.
	if probe, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort)); err != nil {
		return nil, fmt.Errorf("local port %d is already in use", localPort)
	} else {
		probe.Close()
	}

	localBind := fmt.Sprintf("%d:127.0.0.1:%d", localPort, hostPort)

	args := []string{
		"-N",
		"-L", localBind,
		"-l", user,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "BatchMode=yes",
		"-p", port,
		host,
	}

	cmd := exec.Command("ssh", args...)
	cmd.Stdin = nil

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ssh tunnel start: %w", err)
	}

	t := &Tunnel{
		cmd:    cmd,
		stderr: &stderr,
		done:   make(chan struct{}),
	}

	go func() {
		cmd.Wait()
		close(t.done)
	}()

	// Wait until ssh listens on the local port (or dies).
	localAddr := fmt.Sprintf("localhost:%d", localPort)
	for i := 0; i < 50; i++ {
		select {
		case <-t.done:
			errMsg := strings.TrimSpace(stderr.String())
			if errMsg == "" {
				errMsg = "ssh process exited unexpectedly"
			}
			return nil, fmt.Errorf("%s", errMsg)
		default:
		}

		conn, err := net.DialTimeout("tcp", localAddr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return t, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Close()
	errMsg := strings.TrimSpace(stderr.String())
	if errMsg != "" {
		return nil, fmt.Errorf("tunnel timeout: %s", errMsg)
	}
	return nil, fmt.Errorf("tunnel did not become ready within 5s")
}

func newNetnsTunnel(sshHost string, localPort uint16, containerID string, containerPort uint16) (*Tunnel, error) {
	user, addr := parseSSHHost(sshHost)
	host, port := splitHostPort(addr)

	if err := ensureSocatImage(user, host, port); err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return nil, fmt.Errorf("local port %d is already in use", localPort)
	}

	t := &Tunnel{
		listener: listener,
		done:     make(chan struct{}),
		connCmds: make(map[*exec.Cmd]struct{}),
	}

	remoteCmd := fmt.Sprintf(
		"docker run --rm -i --network container:%s %s - TCP:127.0.0.1:%d",
		containerID, socatImage, containerPort,
	)

	go t.acceptLoop(user, host, port, remoteCmd)

	return t, nil
}

func (t *Tunnel) acceptLoop(user, host, port, remoteCmd string) {
	for {
		conn, err := t.listener.Accept()
		if err != nil {
			return
		}
		go t.handleConn(conn, user, host, port, remoteCmd)
	}
}

func (t *Tunnel) handleConn(conn net.Conn, user, host, port, remoteCmd string) {
	defer conn.Close()

	cmd := exec.Command("ssh",
		"-l", user,
		"-p", port,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=/tmp/d4s-ssh-%r@%h-%p",
		"-o", "ControlPersist=60s",
		host,
		remoteCmd,
	)
	cmd.Stdin = conn
	cmd.Stdout = conn

	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.connCmds[cmd] = struct{}{}
	t.mu.Unlock()

	cmd.Run()

	t.mu.Lock()
	delete(t.connCmds, cmd)
	t.mu.Unlock()
}

func ensureSocatImage(user, host, port string) error {
	check := fmt.Sprintf(
		"docker image inspect %s >/dev/null 2>&1 || docker pull %s >/dev/null 2>&1",
		socatImage, socatImage,
	)
	cmd := exec.Command("ssh",
		"-l", user,
		"-p", port,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
		host,
		check,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("cannot prepare %s image on remote: %s", socatImage, msg)
	}
	return nil
}

func (t *Tunnel) Close() {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	cmds := make([]*exec.Cmd, 0, len(t.connCmds))
	for c := range t.connCmds {
		cmds = append(cmds, c)
	}
	t.mu.Unlock()

	if t.listener != nil {
		t.listener.Close()
	}
	for _, c := range cmds {
		if c.Process != nil {
			c.Process.Kill()
		}
	}
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Kill()
		<-t.done
	}
}

func (t *Tunnel) IsRunning() bool {
	if t.cmd != nil {
		select {
		case <-t.done:
			return false
		default:
			return true
		}
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return !t.closed
}

func parseSSHHost(host string) (user, addr string) {
	user = "root"
	addr = host

	addr = strings.TrimSuffix(addr, "/")

	if at := strings.Index(addr, "@"); at >= 0 {
		user = addr[:at]
		addr = addr[at+1:]
	}

	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = addr + ":22"
	}

	return user, addr
}

func splitHostPort(addr string) (string, string) {
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		return addr[:idx], addr[idx+1:]
	}
	return addr, "22"
}
