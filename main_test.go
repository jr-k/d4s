package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Run startup in a subprocess because main uses os.Exit for startup errors.
func TestCLIProcess(t *testing.T) {
	if os.Getenv("D4S_TEST_CLI") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"d4s"}, os.Args[i+1:]...)
			break
		}
	}
	flag.CommandLine = flag.NewFlagSet("d4s", flag.ExitOnError)
	flag.CommandLine.Usage = func() { flag.Usage() }
	main()
	os.Exit(0)
}

func TestConfigFlag(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	invalidPath := filepath.Join(dir, "invalid.yaml")
	if err := os.WriteFile(invalidPath, []byte("d4s: ["), 0o600); err != nil {
		t.Fatal(err)
	}
	missingPath := filepath.Join(dir, "missing.yaml")

	for _, test := range []struct {
		name     string
		args     []string
		wantExit int
		wantText string
	}{
		{"missing file", []string{"--config", missingPath}, 1, missingPath},
		{"invalid file", []string{"--config", invalidPath}, 1, "parse configuration file"},
		{"empty path", []string{"--config="}, 2, "configuration file path cannot be empty"},
		{"help", []string{"--help"}, 0, "--config string"},
		{"version skips config", []string{"--config", missingPath, "--version"}, 0, "Version:"},
	} {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"-test.run=^TestCLIProcess$", "--"}, test.args...)
			command := exec.Command(executable, args...)
			command.Env = append(os.Environ(), "D4S_TEST_CLI=1", "XDG_CONFIG_HOME="+dir)
			output, err := command.CombinedOutput()
			exitCode := 0
			if err != nil {
				exitError, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatal(err)
				}
				exitCode = exitError.ExitCode()
			}
			if exitCode != test.wantExit || !strings.Contains(string(output), test.wantText) {
				t.Fatalf("exit %d, output %q; want exit %d containing %q", exitCode, output, test.wantExit, test.wantText)
			}
		})
	}
}
