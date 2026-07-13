package cmd

import (
	"fmt"
	"os/exec"
	"strings"
)

// CommandExecutor abstracts external command execution for testability.
type CommandExecutor interface {
	Output(name string, args ...string) ([]byte, error)
	Run(name string, args ...string) error
	Start(name string, args ...string) (*exec.Cmd, error)
}

// packageExecutor is the package-level executor used by all commands.
// Replace via SetExecutor in tests to inject mocks.
var packageExecutor CommandExecutor = &OSCommandExecutor{}

// SetExecutor replaces the package-level executor. Use in tests to inject a mock.
func SetExecutor(e CommandExecutor) {
	packageExecutor = e
}

// OSCommandExecutor is the default executor that delegates to os/exec.
type OSCommandExecutor struct{}

// Output runs a command and returns its combined standard output.
func (e *OSCommandExecutor) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// Run runs a command and waits for it to complete.
func (e *OSCommandExecutor) Run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			// Trim any trailing newlines to make the error look cleaner
			cleanOut := strings.TrimSpace(string(out))
			return fmt.Errorf("%w\nOutput: %s", err, cleanOut)
		}

		return err
	}

	return nil
}

// Start starts a command without waiting for it to complete.
func (e *OSCommandExecutor) Start(name string, args ...string) (*exec.Cmd, error) {
	cmd := exec.Command(name, args...)

	return cmd, cmd.Start()
}
