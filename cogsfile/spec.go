package cogsfile

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

const (
	Docker = "docker"
	Shell  = "shell"
)

type Task struct {
	Name         string
	Executor     string
	Image        string
	Shell        string
	ShellArgs    []string          `yaml:"shell_args"`
	EnvVars      map[string]string `yaml:"env_vars"`
	BeforeScript []string          `yaml:"before_script"`
	Script       []string
	AfterScript  []string `yaml:"after_script"`
	DependsOn    []string `yaml:"depends_on"`
}

type Cogsfile struct {
	Tasks []Task
}

func Load(bytes []byte) (*Cogsfile, error) {
	cogsfile := &Cogsfile{}
	err := yaml.Unmarshal(bytes, cogsfile)

	if err != nil {
		return nil, errors.Wrap(err, "unable to parse yaml")
	}

	err = validate(cogsfile)

	if err != nil {
		return nil, errors.Wrap(err, "validation failed")
	}

	return cogsfile, nil
}

func validate(cogsfile *Cogsfile) error {
	if len(cogsfile.Tasks) == 0 {
		return errors.Errorf("one or more tasks required")
	}

	for i, task := range cogsfile.Tasks {
		err := validateTask(task)

		if err != nil {
			return errors.Wrapf(err, "validation failed for task: %s at %d", task.Name, i)
		}
	}

	return nil
}

func validateTask(task Task) error {
	if task.Name == "" {
		return errors.New("name is required")
	}

	if task.Executor != Docker && task.Executor != Shell {
		return errors.Errorf("unsupported executor: %s", task.Executor)
	}

	if task.Executor == Docker && task.Image == "" {
		return errors.New("image is required for docker executor")
	}

	return nil
}
