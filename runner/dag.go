package runner

import (
	"github.com/kinematic-ci/cogs/cogsfile"
	"github.com/kinematic-ci/cogs/list"
	"github.com/pkg/errors"
)

type set map[string]bool

func (s set) add(value string) {
	s[value] = true
}

func (s set) remove(value string) {
	delete(s, value)
}

func ExecutionOrder(tasks []cogsfile.Task, entrypoint string) (*list.TaskList, error) {
	taskMap := map[string]cogsfile.Task{}

	for _, task := range tasks {
		taskMap[task.Name] = task
	}

	entrypointTask, hasEntry := taskMap[entrypoint]

	if !hasEntry {
		return nil, taskNotFound(entrypoint)
	}

	visited := set{}
	discovered := set{}

	order := list.NewTaskList()

	err := visit(order, entrypointTask, taskMap, visited, discovered)

	if err != nil {
		return nil, errors.Wrap(err, "cannot resolve task dependencies")
	}

	return order, nil
}

func taskNotFound(task string) error {
	return errors.Errorf("task '%s' not found", task)
}

func visit(order *list.TaskList, task cogsfile.Task, taskMap map[string]cogsfile.Task, visited, discovered set) error {
	if visited[task.Name] {
		return nil
	}

	if discovered[task.Name] {
		return errors.New("cycle detected in task dependencies")
	}

	discovered.add(task.Name)

	for _, dependency := range task.DependsOn {
		dependencyTask, found := taskMap[dependency]

		if !found {
			return taskNotFound(dependency)
		}

		err := visit(order, dependencyTask, taskMap, visited, discovered)

		if err != nil {
			return err
		}
	}

	discovered.remove(task.Name)
	visited.add(task.Name)
	order.Add(task)

	return nil
}
