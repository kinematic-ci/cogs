package main

import (
	"context"
	"github.com/alexflint/go-arg"
	docker "github.com/docker/docker/client"
	"github.com/kinematic-ci/cogs/cogsfile"
	"github.com/kinematic-ci/cogs/executor"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"log"
	"os"
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

	e := executor.NewDockerExecutor(client, t, cwd)

	defer func() {
		log.Println("Stopping containers")

		err = e.Close(ctx)

		if err != nil {
			log.Fatalln("Error stopping containers", err)
		}
	}()

	log.Println("Creating build")

	log.Println("Executing before_script")
	exitCode, err := runScript(ctx, e, t.BeforeScript)

	if err != nil {
		return errors.Wrap(err, "error executing before_script")
	}

	if exitCode != 0 {
		log.Fatalf("before_script failed with exit code %d\n", exitCode)
	}

	log.Println("Executing script")
	scriptExitCode, err := runScript(ctx, e, t.Script)

	if err != nil {
		return errors.Wrap(err, "error executing script")
	}

	log.Println("Executing after_script")
	exitCode, err = runScript(ctx, e, t.AfterScript)

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

func runScript(ctx context.Context, e executor.Executor, script []string) (int, error) {

	session, err := e.Session(ctx)

	if err != nil {
		return -1, errors.Wrap(err, "unable to create session")
	}

	log.Println("Streaming logs")

	done := make(chan error)

	go func() {
		err = streamOutput(session.Reader())

		if err != nil {
			done <- errors.Wrap(err, "error reading output from container")
		}
		done <- nil
	}()

	for _, cmd := range script {
		err = mustWrite(session.Writer(), cmd)

		if err != nil {
			return -1, errors.Wrap(err, "error executing command")
		}
	}

	err = session.CloseWrite()

	if err != nil {
		return -1, errors.Wrap(err, "error closing IO")
	}

	<-done

	result, err := session.End(ctx)

	if err != nil {
		return -1, errors.Wrap(err, "error ending session")
	}

	return result, nil

}

func streamOutput(reader io.Reader) error {
	size, err := io.Copy(os.Stdout, reader)
	if err != nil {
		return errors.Wrap(err, "error reading from stream")
	}

	log.Printf("Generated %d bytes of log data\n", size)
	return nil
}

func mustWrite(writer io.Writer, str string) error {
	_, err := writer.Write([]byte(str + "\n"))

	if err != nil {
		return errors.Wrap(err, "error writing to stream")
	}
	return nil
}
