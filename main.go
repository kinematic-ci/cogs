package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/alexflint/go-arg"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/kinematic-ci/cogs/cogsfile"
	"github.com/mattn/go-isatty"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/user"
	"runtime"
	"time"

	docker "github.com/docker/docker/client"
)

const (
	perpetualCommand = "sleep"
	defaultTimeout   = "3600"
	ciWorkingDir     = "/ci"
	fallbackUserId   = "0"
)

type arguments struct {
	Spec string `arg:"positional" default:"cogs.yaml"`
}

func main() {

	args := arguments{}

	arg.MustParse(&args)

	bytes, err := ioutil.ReadFile(args.Spec)

	if err != nil {
		log.Fatalln("Error opening Cogsfile:", err)
	}

	cogs, err := cogsfile.Load(bytes)

	if err != nil {
		log.Fatalln("Error parsing Cogsfile", err)
	}

	client, err := docker.NewClientWithOpts(docker.FromEnv)
	if err != nil {
		log.Fatalln("Error creating docker client", err)
	}

	ctx := context.Background()

	err = runCogs(ctx, cogs, client)

	if err != nil {
		log.Fatalln("Task failed", err)
	}

	log.Println("Task completed successfully")
}

func runCogs(ctx context.Context, c *cogsfile.Cogsfile, client *docker.Client) error {
	for _, task := range c.Tasks {
		log.Printf("Executing task %s\n", task.Name)
		err := runTask(ctx, task, client)
		if err != nil {
			return err
		}

	}
	return nil
}

func runTask(ctx context.Context, t cogsfile.Task, client *docker.Client) error {
	log.Println("Starting containers")

	cwd, err := os.Getwd()

	if err != nil {
		return errors.Wrap(err, "cannot determine cwd")
	}

	err = pullImage(ctx, t, client, err)
	if err != nil {
		return errors.Wrap(err, "cannot pull docker image")
	}

	containerName := fmt.Sprintf("%s-%d", t.Name, time.Now().Unix())

	createdContainer, err := client.ContainerCreate(ctx,
		&container.Config{
			User:       userId(),
			Image:      t.Image,
			WorkingDir: ciWorkingDir,
			Cmd:        []string{perpetualCommand, defaultTimeout},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{
				{
					Type:     mount.TypeBind,
					Source:   cwd,
					Target:   ciWorkingDir,
					ReadOnly: false,
				},
			},
		},
		&network.NetworkingConfig{}, containerName)

	if err != nil {
		return err
	}

	err = client.ContainerStart(ctx, createdContainer.ID, types.ContainerStartOptions{})

	if err != nil {
		return errors.Wrap(err, "error starting container")
	}

	defer func() {
		log.Println("Stopping containers")

		timeout := 1 * time.Second
		err = client.ContainerStop(ctx, createdContainer.ID, &timeout)

		if err != nil {
			log.Fatalln("Error stopping containers", err)
		}
	}()

	log.Println("Creating build")

	log.Println("Executing before_script")
	exitCode, err := runScript(err, client, ctx, createdContainer, t.BeforeScript)

	if err != nil {
		return errors.Wrap(err, "error executing before_script")
	}

	if exitCode != 0 {
		log.Fatalf("before_script failed with exit code %d\n", exitCode)
	}

	log.Println("Executing script")
	scriptExitCode, err := runScript(err, client, ctx, createdContainer, t.Script)

	if err != nil {
		return errors.Wrap(err, "error executing script")
	}

	log.Println("Executing after_script")
	exitCode, err = runScript(err, client, ctx, createdContainer, t.AfterScript)

	if err != nil {
		return errors.Wrap(err, "error executing after_script")
	}

	if exitCode != 0 {
		log.Printf("after_script failed with exit code %d\n", exitCode)
	}

	if scriptExitCode != 0 {
		return errors.Errorf("script failed with exit code %d\n", exitCode)
	}

	return nil
}

func pullImage(ctx context.Context, t cogsfile.Task, client *docker.Client, err error) error {
	res, err := client.ImagePull(ctx, t.Image, types.ImagePullOptions{})

	if err != nil {
		return errors.Wrap(err, "cannot pull image from registry")
	}

	err = jsonmessage.DisplayJSONMessagesStream(res, os.Stdout, os.Stdout.Fd(), isatty.IsTerminal(os.Stdout.Fd()), nil)

	if err != nil {
		return errors.Wrap(err, "cannot display progress information for image pull")
	}

	return nil
}

func runScript(err error, client *docker.Client, ctx context.Context, createdContainer container.ContainerCreateCreatedBody, script []string) (int, error) {
	execConfig := types.ExecConfig{
		User:         userId(),
		Privileged:   false,
		Tty:          false,
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
		Env:          nil,
		Cmd:          []string{"/bin/sh", "-xe"},
	}

	execCreated, err := client.ContainerExecCreate(ctx, createdContainer.ID, execConfig)

	if err != nil {
		return -1, errors.Wrap(err, "cannot execute command inside container")
	}

	execAttached, err := client.ContainerExecAttach(ctx, execCreated.ID, types.ExecStartCheck{})

	if err != nil {
		return -1, errors.Wrap(err, "cannot attach to IO of running command")
	}

	log.Println("Streaming logs")

	done := make(chan error)

	go func() {
		err = streamOutput(execAttached.Reader)

		if err != nil {
			done <- errors.Wrap(err, "error reading output from container")
		}
		done <- nil
	}()

	for _, cmd := range script {
		err = mustWrite(execAttached.Conn, cmd)

		if err != nil {
			return -1, errors.Wrap(err, "error executing command")
		}
	}

	err = execAttached.CloseWrite()

	if err != nil {
		return -1, errors.Wrap(err, "error closing IO")
	}

	<-done

	result, err := client.ContainerExecInspect(ctx, execCreated.ID)

	if err != nil {
		return -1, errors.Wrap(err, "error inspecting command execution")
	}

	return result.ExitCode, nil

}

func streamOutput(reader *bufio.Reader) error {
	size, err := io.Copy(os.Stdout, reader)
	if err != nil {
		return errors.Wrap(err, "error reading from stream")
	}

	log.Printf("Generated %d bytes of log data\n", size)
	return nil
}

func mustWrite(conn net.Conn, str string) error {
	_, err := conn.Write([]byte(str + "\n"))

	if err != nil {
		return errors.Wrap(err, "error writing to stream")
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
