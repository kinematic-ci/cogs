package cli

import (
	"fmt"
	"github.com/kinematic-ci/cogs/cogsfile"
	"io/ioutil"
	"log"
	"strings"
)

const colWidth = 25

type TasksArgs struct {
	File string `arg:"-f,--file" help:"Cogsfile for task definitions" default:"cogs.yaml"`
}

func Tasks(args *TasksArgs) {
	bytes, err := ioutil.ReadFile(args.File)

	if err != nil {
		log.Fatalln("Error opening Cogsfile:", err)
	}

	cogs, err := cogsfile.Load(bytes)

	if err != nil {
		log.Fatalln("Error parsing Cogsfile", err)
	}

	println("Available tasks:")
	for _, task := range cogs.Tasks {
		printTwoCols(task.Name, task.Description)
	}
}

func printTwoCols(left, right string) {
	lhs := "  " + left
	fmt.Print(lhs)
	if right != "" {
		if len(lhs)+2 < colWidth {
			fmt.Print(strings.Repeat(" ", colWidth-len(lhs)))
		} else {
			fmt.Print("\n" + strings.Repeat(" ", colWidth))
		}
		fmt.Print(right)
	}
	fmt.Print("\n")
}
