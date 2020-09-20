package executor

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	docker "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"io"
	"log"
	"os"
	"os/user"
	"runtime"
	"time"
)

const (
	perpetualCommand = "sleep"
	defaultTimeout   = "3600"
	ciWorkingDir     = "/ci"
	fallbackUserId   = "0"
)

type dockerSession struct {
	client   *docker.Client
	response types.HijackedResponse
	execID   string
}

func newSession(execID string, response types.HijackedResponse, client *docker.Client) *dockerSession {
	return &dockerSession{client, response, execID}
}

func (s *dockerSession) Reader() io.Reader {
	return s.response.Reader
}

func (s *dockerSession) Writer() io.Writer {
	return s.response.Conn
}

func (s *dockerSession) CloseWrite() error {
	err := s.response.CloseWrite()

	if err != nil {
		return errors.Wrap(err, "error closing session IO")
	}
	return nil
}

func (s *dockerSession) End(ctx context.Context) (int, error) {
	result, err := s.client.ContainerExecInspect(ctx, s.execID)

	if err != nil {
		return -1, errors.Wrap(err, "error inspecting command execution")
	}

	return result.ExitCode, nil
}

type DockerExecutor struct {
	client           *docker.Client
	image            string
	containerID      string
	task             string
	workingDirectory string
	shell            string
	shellArgs        []string
}

func NewDockerExecutor(name, image, workingDirectory, shell string, args []string, client *docker.Client) *DockerExecutor {
	return &DockerExecutor{
		client:           client,
		image:            image,
		task:             name,
		shell:            shell,
		shellArgs:        args,
		workingDirectory: workingDirectory,
	}
}

func (e *DockerExecutor) startContainer(ctx context.Context) error {
	err := pullImage(ctx, e.image, e.client)

	if err != nil {
		return errors.Wrap(err, "cannot pull docker image")
	}

	containerName := fmt.Sprintf("%s-%d", e.task, time.Now().Unix())

	createdContainer, err := e.client.ContainerCreate(ctx,
		&container.Config{
			User:       userId(),
			Image:      e.image,
			WorkingDir: ciWorkingDir,
			Cmd:        []string{perpetualCommand, defaultTimeout},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:     mount.TypeBind,
					Source:   e.workingDirectory,
					Target:   ciWorkingDir,
					ReadOnly: false,
				},
			},
		},
		&network.NetworkingConfig{}, containerName)

	if err != nil {
		return errors.Wrap(err, "error creating container")
	}

	err = e.client.ContainerStart(ctx, createdContainer.ID, types.ContainerStartOptions{})

	if err != nil {
		return errors.Wrap(err, "error starting container")
	}

	e.containerID = createdContainer.ID
	return nil
}

func (e *DockerExecutor) Session(ctx context.Context) (Session, error) {
	if e.containerID == "" {
		err := e.startContainer(ctx)

		if err != nil {
			return nil, errors.Wrap(err, "error creating session")
		}
	}

	cmd := makeCommand(e.shell, e.shellArgs)

	execConfig := types.ExecConfig{
		User:         userId(),
		Privileged:   false,
		Tty:          false,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		Env:          nil,
		Cmd:          cmd,
	}

	execCreated, err := e.client.ContainerExecCreate(ctx, e.containerID, execConfig)

	if err != nil {
		return nil, errors.Wrap(err, "cannot execute command inside container")
	}

	execResponse, err := e.client.ContainerExecAttach(ctx, execCreated.ID, types.ExecStartCheck{})

	return newSession(execCreated.ID, execResponse, e.client), nil
}

func makeCommand(shell string, args []string) []string {
	cmd := []string{shell}

	for _, arg := range args {
		cmd = append(cmd, arg)
	}

	return cmd
}

func (e *DockerExecutor) Close(ctx context.Context) error {
	log.Println("Stopping containers")

	if e.containerID == "" {
		return nil
	}

	timeout := 1 * time.Second
	err := e.client.ContainerStop(ctx, e.containerID, &timeout)

	if err != nil {
		return errors.Wrap(err, "error closing session")
	}

	return nil
}

func pullImage(ctx context.Context, image string, client *docker.Client) error {
	res, err := client.ImagePull(ctx, image, types.ImagePullOptions{})

	if err != nil {
		return errors.Wrap(err, "cannot pull image from registry")
	}

	err = jsonmessage.DisplayJSONMessagesStream(res, os.Stdout, os.Stdout.Fd(), isatty.IsTerminal(os.Stdout.Fd()), nil)

	if err != nil {
		return errors.Wrap(err, "cannot display progress information for image pull")
	}

	return nil
}

func userId() string {
	if runtime.GOOS == "linux" {
		userInfo, err := user.Current()

		if err != nil {
			return fallbackUserId
		}

		return userInfo.Uid
	}

	return fallbackUserId
}
