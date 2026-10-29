package sshutil

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jr-k/d4s/internal/config"
	"github.com/jr-k/d4s/internal/secrets"
)

// ControlMasterArgs returns ssh flags enabling connection multiplexing.
// All ssh invocations for the same host (docker dial-stdio, tunnels,
// remote exec) share a single TCP/SSH connection: one handshake, then
// near-instant channel openings. This also avoids tripping sshd
// MaxStartups when many connections are opened concurrently.
func ControlMasterArgs() []string {
	dir := controlDir()
	if dir == "" {
		return nil
	}
	return []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + filepath.Join(dir, "cm-%r@%h-%p"),
		"-o", "ControlPersist=300s",
	}
}

// ParseSSHHost splits "user@host[:port][/remote/socket/path]" into user
// and "host:port". Any remote path (docker socket) is dropped.
// User defaults to root, port to 22.
func ParseSSHHost(host string) (user, addr string) {
	user = "root"
	addr = host
	if idx := strings.Index(addr, "/"); idx >= 0 {
		addr = addr[:idx]
	}

	if at := strings.Index(addr, "@"); at >= 0 {
		user = addr[:at]
		addr = addr[at+1:]
	}

	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = addr + ":22"
	}

	return user, addr
}

// SplitHostPort splits "host:port", defaulting the port to 22.
func SplitHostPort(addr string) (string, string) {
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		return addr[:idx], addr[idx+1:]
	}
	return addr, "22"
}

// SSHCommand builds an ssh invocation running remoteCmd on sshHost,
// using the stored credentials of contextName (key, passphrase or
// password served via SSH_ASKPASS) and connection multiplexing.
func SSHCommand(contextName, sshHost, remoteCmd string) *exec.Cmd {
	return sshCommand(contextName, sshHost, remoteCmd, false)
}

// SSHCommandTTY is like SSHCommand but forces a pseudo-terminal,
// for interactive remote commands (editors, shells).
func SSHCommandTTY(contextName, sshHost, remoteCmd string) *exec.Cmd {
	return sshCommand(contextName, sshHost, remoteCmd, true)
}

func sshCommand(contextName, sshHost, remoteCmd string, tty bool) *exec.Cmd {
	user, addr := ParseSSHHost(sshHost)
	host, port := SplitHostPort(addr)

	args := []string{
		"-l", user,
		"-p", port,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
	}
	if tty {
		args = append(args, "-tt")
	}

	batchMode := true
	var env []string
	if creds, err := secrets.Load(contextName); err == nil && creds != nil {
		args = append(args, creds.SSHArgs()...)
		if creds.HasSecret() {
			batchMode = false
			env = append(os.Environ(), secrets.AskpassEnv(contextName)...)
		}
	}
	if batchMode {
		args = append(args, "-o", "BatchMode=yes")
	}

	args = append(args, ControlMasterArgs()...)
	args = append(args, host, remoteCmd)

	cmd := exec.Command("ssh", args...)
	if env != nil {
		cmd.Env = env
	}
	return cmd
}

// CloseControlMaster terminates the multiplexed ssh master connection for
// sshHost, if any. The next ssh invocation opens a fresh session, which is
// required for server-side changes (group membership, authorized keys)
// to be picked up.
func CloseControlMaster(sshHost string) {
	dir := controlDir()
	if dir == "" || sshHost == "" {
		return
	}
	user, addr := ParseSSHHost(sshHost)
	host, port := SplitHostPort(addr)
	args := []string{
		"-O", "exit",
		"-o", "ControlPath=" + filepath.Join(dir, "cm-%r@%h-%p"),
		"-l", user,
		"-p", port,
		host,
	}
	_ = exec.Command("ssh", args...).Run()
}

// ShellQuote quotes s for safe interpolation in a remote shell command.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// controlDir returns a private (0700) directory for control sockets.
func controlDir() string {
	base := config.ConfigDir()
	if base == "" {
		return ""
	}
	dir := filepath.Join(base, "ssh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	// The config dir itself may be 0755; the ssh subdir must stay private.
	if err := os.Chmod(dir, 0o700); err != nil {
		return ""
	}
	return dir
}
