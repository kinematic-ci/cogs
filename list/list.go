package list

import "github.com/kinematic-ci/cogs/cogsfile"

type TaskList struct {
	values []cogsfile.Task
}

func NewTaskList(values ...cogsfile.Task) *TaskList {
	return &TaskList{values}
}

func (d *TaskList) Add(value cogsfile.Task) {
	d.values = append(d.values, value)
}

func (d *TaskList) Values() []cogsfile.Task {
	return d.values
}
