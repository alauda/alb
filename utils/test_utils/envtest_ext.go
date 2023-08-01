package test_utils

import (
	"os"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type EnvtestExt struct {
	env  *envtest.Environment
	cfg  *rest.Config
	log  logr.Logger
	base string
}

func NewEnvtestExt(base string, l logr.Logger) *EnvtestExt {
	return &EnvtestExt{env: &envtest.Environment{}, log: l, base: base}
}

func (e *EnvtestExt) Start() (*rest.Config, error) {
	cfg, err := e.env.Start()
	if err != nil {
		return nil, err
	}
	e.cfg = cfg
	err = e.init()
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
func (e *EnvtestExt) AssertStart() *rest.Config {
	cfg, err := e.Start()
	if err == nil {
		return cfg
	}
	panic(err)
}

func (e *EnvtestExt) init() error {
	albRoot := os.Getenv("ALB_ROOT")
	err := InitCrd(albRoot, e.cfg)
	if err != nil {
		return err
	}
	return nil
}

func (e *EnvtestExt) Stop() error {
	return e.env.Stop()
}
