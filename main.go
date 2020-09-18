package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/alexflint/go-arg"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	docker "github.com/docker/docker/client"
)

const (
	perpetualCommand = "sleep"
	defaultTimeout   = "3600"
	defaultUserID    = "1000"
)

type task struct {
	Name         string
	Image        string
	EnvVars      map[string]string
	BeforeScript []string
	Script       []string
	AfterScript  []string
}

type cogsFile struct {
	Tasks []task
}

type arguments struct {
	Spec string `arg:"positional" default:"cogs.yaml"`
}

func main() {

	args := arguments{}

	arg.MustParse(&args)

	specFile, err := ioutil.ReadFile(args.Spec)

	if err != nil {
		log.Fatalln("Error opening Cogsfile:", err)
	}

	var cogs cogsFile

	err = yaml.Unmarshal(specFile, &cogs)

	if err != nil {
		log.Fatalln("Error parsing yaml", err)
	}

	client, err := docker.NewEnvClient()
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

func runCogs(ctx context.Context, k cogsFile, client *docker.Client) error {
	for _, task := range k.Tasks {
		log.Printf("Executing task %s\n", task.Name)
		err := runTask(ctx, task, client)
		if err != nil {
			return err
		}

	}
	return nil
}

func runTask(ctx context.Context, t task, client *docker.Client) error {
	log.Println("Starting containers")

	containerConfig := &container.Config{
		User:  defaultUserID,
		Image: t.Image,
		Cmd:   []string{perpetualCommand, defaultTimeout},
	}

	containerName := fmt.Sprintf("%s-%d", t.Name, time.Now().Unix())
	createdContainer, err := client.ContainerCreate(ctx,
		containerConfig,
		&container.HostConfig{},
		&network.NetworkingConfig{}, containerName)

	if err != nil {
		return err
	}

	err = client.ContainerStart(ctx, createdContainer.ID, types.ContainerStartOptions{})

	if err != nil {
		return errors.Wrap(err, "Error starting container")
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
		return errors.Wrap(err, "Error executing before_script")
	}

	if exitCode != 0 {
		log.Fatalf("before_script failed with exit code %d\n", exitCode)
	}

	log.Println("Executing before_script")
	scriptExitCode, err := runScript(err, client, ctx, createdContainer, t.Script)

	if err != nil {
		return errors.Wrap(err, "Error executing script")
	}

	log.Println("Executing before_script")
	exitCode, err = runScript(err, client, ctx, createdContainer, t.AfterScript)

	if err != nil {
		return errors.Wrap(err, "Error executing after_script")
	}

	if exitCode != 0 {
		log.Printf("after_script failed with exit code %d\n", exitCode)
	}

	if scriptExitCode != 0 {
		return errors.Errorf("script failed with exit code %d\n", exitCode)
	}

	return nil
}

func runScript(err error, client *docker.Client, ctx context.Context, createdContainer container.ContainerCreateCreatedBody, script []string) (int, error) {
	execConfig := types.ExecConfig{
		User:         "1000",
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
		return -1, errors.Wrap(err, "Cannot execute command inside container")
	}

	execAttached, err := client.ContainerExecAttach(ctx, execCreated.ID, execConfig)

	if err != nil {
		return -1, errors.Wrap(err, "Cannot attach to IO of running command")
	}

	log.Println("Streaming logs")

	done := make(chan error)

	go func() {
		err = streamOutput(execAttached.Reader)

		if err != nil {
			done <- errors.Wrap(err, "Error reading output from container")
		}
		done <- nil
	}()

	for _, cmd := range script {
		err = mustWrite(execAttached.Conn, cmd)

		if err != nil {
			return -1, errors.Wrap(err, "Error executing command")
		}
	}

	err = execAttached.CloseWrite()

	if err != nil {
		return -1, errors.Wrap(err, "Error closing IO")
	}

	<-done

	result, err := client.ContainerExecInspect(ctx, execCreated.ID)

	if err != nil {
		return -1, errors.Wrap(err, "Error inspecting command execution")
	}

	return result.ExitCode, nil

}

func streamOutput(reader *bufio.Reader) error {
	size, err := io.Copy(os.Stdout, reader)
	if err != nil {
		return errors.Wrap(err, "Error reading from stream")
	}

	log.Printf("Generated %d bytes of log data\n", size)
	return nil
}

func mustWrite(conn net.Conn, str string) error {
	_, err := conn.Write([]byte(str + "\n"))

	if err != nil {
		return errors.Wrap(err, "Error writing to stream")
	}
	return nil
}
