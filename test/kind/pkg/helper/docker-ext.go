package helper

import (
	"fmt"
	"strings"
	"unicode"

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

func (d *DockerExt) PullIfNotFound(images ...string) error {
	for _, image := range images {
		err := d.pullIfNotFound(image)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *DockerExt) pullIfNotFound(image string) error {
	has, err := d.HasImage(image)
	if err != nil {
		return err
	}
	if !has {
		d.Pull(image)
	}
	return nil
}

func (d *DockerExt) HasImage(imgWithTag string) (bool, error) {
	img, tag, err := splitImageTag(imgWithTag)
	d.log.Info("has", "img", img, "tag", tag)
	if err != nil {
		return false, err
	}
	out, err := NewCmd().Logout(false).Call("docker", "images")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.FieldsFunc(line, func(r rune) bool {
			return unicode.IsSpace(r) || r == '\t'
		})
		if len(fields) < 2 {
			continue
		}
		if fields[0] == img && fields[1] == tag {
			return true, nil
		}
	}
	return false, nil
}

func splitImageTag(imgWithTag string) (string, string, error) {
	lastInd := strings.LastIndex(imgWithTag, ":")
	if lastInd == -1 {
		return "", "", fmt.Errorf("invalid format %v", lastInd)
	}

	return imgWithTag[:lastInd], imgWithTag[lastInd+1:], nil
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
