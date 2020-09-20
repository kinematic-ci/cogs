package runner

import (
	"github.com/kinematic-ci/cogs/cogsfile"
	"github.com/kinematic-ci/cogs/list"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExecutionOrder(t *testing.T) {
	t.Run("Should resolve single task", func(t *testing.T) {
		tasks := []cogsfile.Task{{Name: "build"}}

		actual, err := ExecutionOrder(tasks, "build")

		expected := list.NewTaskList(cogsfile.Task{Name: "build"})

		assert.Nil(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("Should return error if entrypoint is not found", func(t *testing.T) {
		tasks := []cogsfile.Task{{Name: "build"}}

		actual, err := ExecutionOrder(tasks, "nonexistent")

		assert.Error(t, err, "task 'nonexistent' not found")
		assert.Nil(t, actual)
	})

	t.Run("Should ignore tasks which are not required", func(t *testing.T) {
		tasks := []cogsfile.Task{{Name: "build"}, {Name: "test"}}

		actual, err := ExecutionOrder(tasks, "build")

		expected := list.NewTaskList(cogsfile.Task{Name: "build"})

		assert.Nil(t, err)
		assert.Equal(t, expected, actual)
	})

	t.Run("Should return error if task specifies nonexistent dependencies", func(t *testing.T) {
		tasks := []cogsfile.Task{
			{Name: "build", DependsOn: []string{"check"}},
			{Name: "check", DependsOn: []string{"setup"}},
			{Name: "test"}}

		actual, err := ExecutionOrder(tasks, "build")

		assert.Nil(t, actual)
		assert.NotNil(t, err)
		assert.Equal(t, err.Error(), "cannot resolve task dependencies: task 'setup' not found")
	})

	t.Run("Should return error if task specifies circular dependencies", func(t *testing.T) {
		tasks := []cogsfile.Task{
			{Name: "build", DependsOn: []string{"check"}},
			{Name: "check", DependsOn: []string{"test"}},
			{Name: "check", DependsOn: []string{"build"}},
		}

		actual, err := ExecutionOrder(tasks, "build")

		assert.Nil(t, actual)
		assert.NotNil(t, err)
		assert.Equal(t, err.Error(), "cannot resolve task dependencies: cycle detected in task dependencies")
	})

	t.Run("Should include tasks in order of dependencies", func(t *testing.T) {
		tasks := []cogsfile.Task{
			{Name: "compile"},
			{Name: "test", DependsOn: []string{"compile"}},
			{Name: "integration_test", DependsOn: []string{"compile"}},
			{Name: "bundle", DependsOn: []string{"test", "integration_test"}},
		}

		actual, err := ExecutionOrder(tasks, "bundle")

		expected := list.NewTaskList(
			cogsfile.Task{Name: "compile"},
			cogsfile.Task{Name: "test", DependsOn: []string{"compile"}},
			cogsfile.Task{Name: "integration_test", DependsOn: []string{"compile"}},
			cogsfile.Task{Name: "bundle", DependsOn: []string{"test", "integration_test"}},
		)

		assert.Nil(t, err)
		assert.Equal(t, expected, actual)
	})
}
