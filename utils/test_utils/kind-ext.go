package test_utils

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
)

type KindExt struct {
	log  logr.Logger
	base string
	Name string
}

type KindConfig struct {
	Base        string
	Name        string
	Image       string
	ClusterYaml string
}

func KindDelete(name string) error {
	_, err := Command("kind", "delete", "cluster", "--name", name)
	if err != nil {
		return err
	}
	return nil
}

func KindLs() ([]string, error) {
	out, err := Command("kind", "get", "clusters")
	if err != nil {
		return nil, err
	}
	if strings.Contains(out, "No kind clusters found") {
		return []string{}, nil
	}

	lines := strings.Split(out, "\n") // split the string into lines
	var nonEmptyLines []string
	for _, line := range lines {
		if line != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}
	return nonEmptyLines, nil
}

func AdoptKind(base string, name string, log logr.Logger) *KindExt {
	return &KindExt{
		log:  log,
		base: base,
		Name: name,
	}
}

func DeployOrAdopt(cfg KindConfig, base string, prefix string, log logr.Logger) (*KindExt, error) {
	ls, err := KindLs()
	if err != nil {
		return nil, err
	}
	name := ""
	for _, k := range ls {
		if strings.HasPrefix(k, prefix) {
			name = k
			break
		}
	}
	if name == "" {
		log.Info("create kind cluster", "name", cfg.Name, "cfg", cfg.ClusterYaml)
		return DeployKind(cfg, base, log)
	}
	log.Info("adopt kind", "name", name)
	return &KindExt{
		log:  log,
		base: base,
		Name: name,
	}, nil
}

func DeployKind(cfg KindConfig, base string, log logr.Logger) (*KindExt, error) {
	base = base + "/" + "kind"
	err := os.MkdirAll(base, 0777)
	if err != nil {
		return nil, err
	}
	kindcluster := base + "/kind-cluster.yaml"
	err = os.WriteFile(kindcluster, []byte(cfg.ClusterYaml), 0666)
	if err != nil {
		return nil, err
	}

	_, err = Command("kind", "create", "cluster", "--config", kindcluster, "--name", cfg.Name, "--image", cfg.Image)
	if err != nil {
		return nil, err
	}
	return &KindExt{
		log:  log,
		base: base,
		Name: cfg.Name,
	}, nil
}

func (k *KindExt) SetLogger(log logr.Logger) {
	k.log = log
}

func (k *KindExt) LoadImage(imags ...string) error {
	for _, imag := range imags {
		k.log.Info("load image", "image", imag)
		_, err := Command("kind", "load", "docker-image", imag, "--name", k.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (k *KindExt) GetConfig() (*rest.Config, error) {
	cfgRaw, err := Command("kind", "get", "kubeconfig", "--name", k.Name)
	if err != nil {
		return nil, err
	}

	r, err := RESTFromKubeConfig(cfgRaw)
	if err != nil {
		return nil, fmt.Errorf("get config from raw fail %v", err)
	}
	return r, nil

}

func (k *KindExt) ExecInDocker(cmd string) (string, error) {
	return Command("docker", "exec", k.Name+"-control-plane", "bash", "-c", cmd)
}
