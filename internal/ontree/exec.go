package ontree

import (
	"context"
	"os/exec"
)

type execCmd struct {
	cmd *exec.Cmd
}

func (e *execCmd) Output() ([]byte, error) {
	return e.cmd.Output()
}

func (e *execCmd) CombinedOutput() ([]byte, error) {
	return e.cmd.CombinedOutput()
}

func (e *execCmd) Start() error {
	return e.cmd.Start()
}

func (e *execCmd) Wait() error {
	return e.cmd.Wait()
}

func (e *execCmd) StdoutPipe() (readCloser, error) {
	return e.cmd.StdoutPipe()
}

func (e *execCmd) StderrPipe() (readCloser, error) {
	return e.cmd.StderrPipe()
}

func (m *Manager) defaultExecCommand(ctx context.Context, name string, args ...string) commandRunner {
	return &execCmd{cmd: exec.CommandContext(ctx, name, args...)} //nolint:gosec // command args are validated by callers
}
