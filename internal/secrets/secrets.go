package secrets

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

const service = "d4s"

const (
	AuthTypeKey      = "key"
	AuthTypePassword = "password"
)

// Env vars used to make d4s act as an SSH_ASKPASS helper.
const (
	askpassEnvFlag    = "D4S_ASKPASS"
	askpassEnvContext = "D4S_ASKPASS_CONTEXT"
)

// SSHCredentials holds the authentication settings of a remote SSH context.
// Passphrase/Password are stored in the OS keychain (macOS Keychain,
// Windows Credential Manager, Linux Secret Service).
type SSHCredentials struct {
	AuthType   string `json:"auth_type"`
	KeyPath    string `json:"key_path,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
	Password   string `json:"password,omitempty"`
}

// Save stores credentials in the OS keyring (macOS Keychain, Windows
// Credential Manager, Linux Secret Service). When no keyring is available
// (headless Linux), it falls back to an encrypted file in the d4s config dir.
func Save(contextName string, creds SSHCredentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return err
	}
	if err := keyring.Set(service, contextName, string(data)); err != nil {
		return fileStoreSave(contextName, creds)
	}
	return nil
}

func Load(contextName string) (*SSHCredentials, error) {
	data, err := keyring.Get(service, contextName)
	if err == nil {
		var creds SSHCredentials
		if jerr := json.Unmarshal([]byte(data), &creds); jerr != nil {
			return nil, jerr
		}
		return &creds, nil
	}
	// Not in keyring (or keyring unavailable): try the encrypted file store.
	return fileStoreLoad(contextName)
}

func Delete(contextName string) {
	_ = keyring.Delete(service, contextName)
	fileStoreDelete(contextName)
}

// Secret returns the secret ssh should receive when prompting
// (key passphrase or login password).
func (c *SSHCredentials) Secret() string {
	if c.AuthType == AuthTypePassword {
		return c.Password
	}
	return c.Passphrase
}

// HasSecret reports whether ssh will need a non-interactive answer.
func (c *SSHCredentials) HasSecret() bool {
	return c != nil && c.Secret() != ""
}

// SSHArgs returns extra ssh CLI flags for these credentials.
func (c *SSHCredentials) SSHArgs() []string {
	if c == nil {
		return nil
	}
	var args []string
	if c.AuthType == AuthTypeKey && c.KeyPath != "" {
		args = append(args, "-i", c.KeyPath, "-o", "IdentitiesOnly=yes")
	}
	if c.AuthType == AuthTypePassword {
		args = append(args, "-o", "PubkeyAuthentication=no")
	}
	return args
}

// AskpassEnv returns environment variables that make any spawned ssh
// process use d4s itself as the askpass helper for this context.
func AskpassEnv(contextName string) []string {
	exe, err := os.Executable()
	if err != nil {
		return nil
	}
	return []string{
		fmt.Sprintf("SSH_ASKPASS=%s", exe),
		"SSH_ASKPASS_REQUIRE=force",
		fmt.Sprintf("%s=1", askpassEnvFlag),
		fmt.Sprintf("%s=%s", askpassEnvContext, contextName),
		"DISPLAY=:0",
	}
}

// ApplyAskpassEnv sets the askpass environment on the current process so
// that ssh children spawned indirectly (e.g. by docker connhelper)
// inherit it. Call with empty name to clear.
func ApplyAskpassEnv(contextName string) {
	if contextName == "" {
		os.Unsetenv("SSH_ASKPASS")
		os.Unsetenv("SSH_ASKPASS_REQUIRE")
		os.Unsetenv(askpassEnvFlag)
		os.Unsetenv(askpassEnvContext)
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	os.Setenv("SSH_ASKPASS", exe)
	os.Setenv("SSH_ASKPASS_REQUIRE", "force")
	os.Setenv(askpassEnvFlag, "1")
	os.Setenv(askpassEnvContext, contextName)
	if os.Getenv("DISPLAY") == "" {
		os.Setenv("DISPLAY", ":0")
	}
}

// RunAskpassIfRequested makes d4s act as an SSH_ASKPASS helper: when ssh
// invokes us with the askpass env set, print the stored secret and exit.
// Returns true if the process handled an askpass request.
func RunAskpassIfRequested() bool {
	if os.Getenv(askpassEnvFlag) != "1" {
		return false
	}
	ctxName := os.Getenv(askpassEnvContext)
	if ctxName == "" {
		return true
	}
	creds, err := Load(ctxName)
	if err != nil || creds == nil {
		return true
	}
	fmt.Println(creds.Secret())
	return true
}
