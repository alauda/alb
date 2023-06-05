package helper

import (
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"

	_ "embed"

	. "alauda.io/alb2/utils/test_utils"
)

//go:embed echo-resty.yaml
var EchoRestyTemplate string

type Echo struct {
	log     logr.Logger
	albroot string
	base    string
	k       *Kubectl
}

func NewEchoResty(base string, cfg *rest.Config, log logr.Logger) *Echo {
	return &Echo{
		log:  log,
		base: base,
		k:    NewKubectl(base, cfg, log),
	}
}

func (e *Echo) enable() error {
	return nil
}

type EchoCfg struct {
	Image string
	Name  string
	Ip    string
	Lb    string
}

func (e *Echo) Deploy(cfg EchoCfg) error {
	// k := e.k
	echo := Template(EchoRestyTemplate, map[string]interface{}{
		"Values": map[string]interface{}{
			"image":    cfg.Image,
			"name":     cfg.Name,
			"ip":       cfg.Ip,
			"replicas": 1,
		},
	})
	e.log.Info("echo", "yaml", echo)
	out, err := e.k.KubectlApply(echo)
	if err != nil {
		return err
	}
	e.log.Info(out)
	return nil
}
