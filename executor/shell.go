package executor

import (
	"context"
	"github.com/pkg/errors"
	"io"
	"os/exec"
)

type shellSession struct {
	cmd    *exec.Cmd
	stdout io.Reader
	stdin  io.WriteCloser
}

func newShellSession(cmd *exec.Cmd) (*shellSession, error) {
	stdout, err := cmd.StdoutPipe()

	if err != nil {
		return nil, errors.Wrap(err, "unable to pipe STDOUT")
	}

	stdin, err := cmd.StdinPipe()

	if err != nil {
		return nil, errors.Wrap(err, "unable to pipe STDIN")
	}

	err = cmd.Start()

	if err != nil {
		return nil, errors.Wrap(err, "unable to start shell")
	}

	return &shellSession{
		cmd:    cmd,
		stdout: stdout,
		stdin:  stdin,
	}, nil
}

func (s *shellSession) Reader() io.Reader {
	return s.stdout
}

func (s *shellSession) Writer() io.Writer {
	return s.stdin
}

func (s *shellSession) CloseWrite() error {
	err := s.stdin.Close()

	if err != nil {
		return errors.Wrap(err, "unable to close IO")
	}

	return nil
}

func (s *shellSession) End(_ context.Context) (int, error) {
	err := s.cmd.Wait()

	if err != nil {
		return -1, errors.Wrap(err, "error while waiting for process to end")
	}

	return s.cmd.ProcessState.ExitCode(), nil
}

type ShellExecutor struct {
	Shell            string
	ShellArguments   []string
	WorkingDirectory string
}

func (s *ShellExecutor) Name() string {
	return "shell"
}

func NewShellExecutor(workingDirectory, shell string, shellArguments []string) *ShellExecutor {
	return &ShellExecutor{WorkingDirectory: workingDirectory, Shell: shell, ShellArguments: shellArguments}
}

func (s *ShellExecutor) Session(_ context.Context) (Session, error) {
	cmd := exec.Command(s.Shell, s.ShellArguments...)
	cmd.Dir = s.WorkingDirectory
	session, err := newShellSession(cmd)

	if err != nil {
		return nil, errors.Wrap(err, "unable to start session")
	}

	return session, nil
}

func (s *ShellExecutor) Close(_ context.Context) error {
	return nil
}
