package cmd

import "os/exec"

// CommandExecutor abstracts external command execution for testability.
type CommandExecutor interface {
	Output(name string, args ...string) ([]byte, error)
	Run(name string, args ...string) error
	Start(name string, args ...string) (*exec.Cmd, error)
}

// OSCommandExecutor is the default executor that delegates to os/exec.
type OSCommandExecutor struct{}

// Output runs a command and returns its combined standard output.
func (e *OSCommandExecutor) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// Run runs a command and waits for it to complete.
func (e *OSCommandExecutor) Run(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

// Start starts a command without waiting for it to complete.
func (e *OSCommandExecutor) Start(name string, args ...string) (*exec.Cmd, error) {
	cmd := exec.Command(name, args...)

	return cmd, cmd.Start()
}
