package cogsfile

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("Should load valid Cogsfile", func(t *testing.T) {

		c, err := Load([]byte((
			`
tasks:
  - name: build
    executor: docker
    image: 'docker.io/library/python:3'
    env_vars:
      FOO: BAR
    before_script:
      - pwd
    script:
      - python -c 'print("Hello World")'
    after_script:
      - echo 'Done'`)))

		expected := &Cogsfile{Tasks: []Task{
			{
				Name:     "build",
				Executor: "docker",
				Image:    "docker.io/library/python:3",
				EnvVars: map[string]string{
					"FOO": "BAR",
				},
				BeforeScript: []string{"pwd"},
				Script:       []string{`python -c 'print("Hello World")'`},
				AfterScript:  []string{`echo 'Done'`},
			},
		}}

		assert.Nil(t, err)
		assert.Equal(t, expected, c)

	})

	t.Run("Should return error on invalid yaml", func(t *testing.T) {
		c, err := Load([]byte((`task`)))
		assert.Nil(t, c)
		assert.NotNil(t, err)
		assert.Equal(t,
			"unable to parse yaml: yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `task` into cogsfile.Cogsfile",
			err.Error())
	})

	t.Run("Should return error if no tasks are present", func(t *testing.T) {
		c, err := Load([]byte((`tasks: []`)))
		assert.Nil(t, c)
		assert.NotNil(t, err)
		assert.Equal(t,
			"validation failed: one or more tasks required",
			err.Error())
	})

	t.Run("Should return error if task does not have name", func(t *testing.T) {
		c, err := Load([]byte((
			`
tasks:
  - executor: docker
    image: 'docker.io/library/python:3'
    env_vars:
      FOO: BAR
    before_script:
      - pwd
    script:
      - python -c 'print("Hello World")'
    after_script:
      - echo 'Done'`)))

		assert.Nil(t, c)
		assert.NotNil(t, err)
		assert.Equal(t,
			"validation failed: validation failed for task:  at 0: name is required",
			err.Error())
	})

	t.Run("Should return error if executor is unsupported", func(t *testing.T) {
		c, err := Load([]byte((
			`
tasks:
  - name: build
    executor: blah
    image: 'docker.io/library/python:3'
    env_vars:
      FOO: BAR
    before_script:
      - pwd
    script:
      - python -c 'print("Hello World")'
    after_script:
      - echo 'Done'`)))

		assert.Nil(t, c)
		assert.NotNil(t, err)
		assert.Equal(t,
			"validation failed: validation failed for task: build at 0: unsupported executor: blah",
			err.Error())
	})

	t.Run("Should return error if image is missing for a task with docker executor", func(t *testing.T) {
		c, err := Load([]byte((
			`
tasks:
  - name: build
    executor: docker
    env_vars:
      FOO: BAR
    before_script:
      - pwd
    script:
      - python -c 'print("Hello World")'
    after_script:
      - echo 'Done'`)))

		assert.Nil(t, c)
		assert.NotNil(t, err)
		assert.Equal(t,
			"validation failed: validation failed for task: build at 0: image is required for docker executor",
			err.Error())
	})
}
