package main

import (
	"github.com/alexflint/go-arg"
	"github.com/kinematic-ci/cogs/cli"
	"log"
	"os"
)

func main() {
	type arguments struct {
		Run *cli.RunArgs `arg:"subcommand:run" help:"Run a target"`
	}

	log.SetPrefix("[⚙️ ] ")

	args := arguments{}

	p := arg.MustParse(&args)

	switch {
	case args.Run != nil:
		cli.Run(args.Run)
	default:
		if len(os.Args) >= 2 {
			cli.Run(&cli.RunArgs{
				Target: os.Args[2],
			})
		} else {
			p.Fail("invalid subcommand")
		}
	}
}
