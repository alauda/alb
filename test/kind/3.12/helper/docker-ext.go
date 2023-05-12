package helper

import (
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"golang.org/x/exp/slices"
)

type DockerExt struct {
	log logr.Logger
}

func NewDockerExt(log logr.Logger) DockerExt {
	return DockerExt{log: log}
}

func (d *DockerExt) Pull(images ...string) error {
	for _, image := range images {
		_, err := Command("docker", "pull", image)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DockerExt) Build(cmds ...string) (string, error) {
	out, err := Command("docker ", append([]string{"build"}, cmds...)...)
	if err != nil {
		return "", err
	}
	// find line which contains special substring
	lines := strings.Split(out, "\n")
	index := slices.IndexFunc(lines, func(s string) bool {
		return strings.Contains(s, "Successfully tagged")
	})
	if index == -1 {
		return "", fmt.Errorf("could not find image tag")
	}
	tagStr := strings.TrimSpace(lines[index])
	tagStr = strings.Trim(tagStr, "\x00")
	return strings.Fields(tagStr)[2], nil
}
