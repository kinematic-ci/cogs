package list

import (
	"github.com/kinematic-ci/cogs/cogsfile"
	"gotest.tools/assert"
	"testing"
)

func TestTaskList_Add(t *testing.T) {
	list := NewTaskList()

	list.Add(cogsfile.Task{
		Name: "build",
	})
	list.Add(cogsfile.Task{
		Name: "test",
	})

	expected := []cogsfile.Task{{Name: "build"}, {Name: "test"}}

	assert.DeepEqual(t, expected, list.Values())
}

func TestTaskList_Values(t *testing.T) {
	list := NewTaskList(cogsfile.Task{
		Name: "build",
	}, cogsfile.Task{
		Name: "test",
	})

	expected := []cogsfile.Task{{Name: "build"}, {Name: "test"}}

	assert.DeepEqual(t, expected, list.Values())
}
