package main

import (
	"github.com/alexflint/go-arg"
	"github.com/kinematic-ci/cogs/cli"
	"github.com/kinematic-ci/cogs/cogsfile"
	"log"
	"os"
)

func main() {
	type arguments struct {
		Run *cli.RunArgs `arg:"subcommand:run" help:"Run a target"`
	}

	log.SetPrefix("[⚙️ ] ")

	args := arguments{}

	arg.MustParse(&args)

	switch {
	case args.Run != nil:
		cli.Run(args.Run)
	default:
		fallbackToRun()
	}
}

func fallbackToRun() {
	var target string
	switch {
	case len(os.Args) == 1:
		target = ""
	case len(os.Args) >= 2:
		target = os.Args[1]

	}
	cli.Run(&cli.RunArgs{
		Target: target,
		File:   cogsfile.DefaultFileName,
	})
}
