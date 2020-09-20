package main

import (
	"context"
	"github.com/alexflint/go-arg"
	docker "github.com/docker/docker/client"
	"github.com/kinematic-ci/cogs/cogsfile"
	"github.com/kinematic-ci/cogs/executor"
	"github.com/kinematic-ci/cogs/runner"
	"github.com/kinematic-ci/cogs/utils"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"log"
	"os"
)

type options struct {
	alwaysDocker bool
	alwaysShell  bool
}

func main() {
	type arguments struct {
		Target       string `arg:"positional" default:""`
		Spec         string `default:"cogs.yaml"`
		AlwaysDocker bool   `arg:"-d,--always-docker" help:"Always use Docker executor"`
		AlwaysShell  bool   `arg:"-s,--always-shell" help:"Always use Shell executor"`
	}

	log.SetPrefix("[⚙️ ] ")

	args := arguments{}

	arg.MustParse(&args)

	opts := options{
		alwaysDocker: args.AlwaysDocker,
		alwaysShell:  args.AlwaysShell,
	}

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

	err = runCogs(ctx, cogs, args.Target, opts, client)

	if err != nil {
		log.Fatalln("Task failed", err)
	}

	log.Println("Task completed successfully")
}

func runCogs(ctx context.Context, c *cogsfile.Cogsfile, target string, opts options, client *docker.Client) error {
	if target == "" {
		target = c.Tasks[0].Name
	}

	taskList, err := runner.ExecutionOrder(c.Tasks, target)

	if err != nil {
		return errors.Wrap(err, "unable to determine execution order")
	}

	for _, task := range taskList.Values() {
		log.Printf("Executing task %s\n", task.Name)
		err := runTask(ctx, task, opts, client)

		if err != nil {
			return errors.Wrap(err, "error executing task")
		}
	}

	return nil
}

const defaultShell = "/bin/sh"

func runTask(ctx context.Context, t cogsfile.Task, opts options, client *docker.Client) error {
	cwd, err := os.Getwd()

	if err != nil {
		return errors.Wrap(err, "cannot determine cwd")
	}

	shell := utils.StringOrDefault(t.Shell, defaultShell)
	shellArgs := getShellArgs(t.ShellArgs)

	var e executor.Executor

	if opts.alwaysDocker {
		log.Println("Overriding executor to use docker")
		e = executor.NewDockerExecutor(t.Name, t.Image, cwd, shell, shellArgs, client)
	} else if opts.alwaysShell {
		log.Println("Overriding executor to use shell")
		e = executor.NewShellExecutor(cwd, shell, shellArgs)
	} else {
		switch t.Executor {
		case cogsfile.Docker:
			e = executor.NewDockerExecutor(t.Name, t.Image, cwd, shell, shellArgs, client)
		case cogsfile.Shell:
			e = executor.NewShellExecutor(cwd, shell, shellArgs)
		default:
			log.Panicf("Unknown executor: %s\n", t.Executor)
		}
	}

	log.Printf("Using executor: %s\n", e.Name())

	defer func() {
		log.Println("Closing executor")

		err = e.Close(ctx)

		if err != nil {
			log.Fatalln("Error closing executor", err)
		}
	}()

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

func getShellArgs(args []string) []string {
	combinedArgs := []string{"-xe"}

	for _, a := range args {
		combinedArgs = append(combinedArgs, a)
	}

	return combinedArgs
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
