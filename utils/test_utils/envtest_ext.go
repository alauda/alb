package test_utils

import (
	"os"
	"path"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type EnvtestExt struct {
	env        *envtest.Environment
	Name       string
	cfg        *rest.Config
	cfgpath    string
	crds       []string
	log        logr.Logger
	Base       string
	initForAlb bool
}

func NewEnvtestExt(base string, l logr.Logger) *EnvtestExt {
	return &EnvtestExt{env: &envtest.Environment{}, log: l, Base: base, initForAlb: true}
}

func (e *EnvtestExt) WithName(name string) *EnvtestExt {
	e.Name = name
	return e
}

func (e *EnvtestExt) Crds(crds []string) *EnvtestExt {
	// e.env.CRDDirectoryPaths = append(e.env.CRDDirectoryPaths, crds...)
	e.crds = crds
	return e
}

func (e *EnvtestExt) NotInitAlbCr() *EnvtestExt {
	e.initForAlb = false
	return e
}
func (e *EnvtestExt) Kubectl() *Kubectl {
	return NewKubectl(e.Base, e.cfg, e.log.WithName("kubctl"))
}

func (e *EnvtestExt) GetKubeCfgPath() string {
	return e.cfgpath
}
func (e *EnvtestExt) GetRestCfg() *rest.Config {
	return e.cfg
}

func (e *EnvtestExt) Start() (*rest.Config, error) {
	cfg, err := e.env.Start()
	if err != nil {
		return nil, err
	}
	e.cfg = cfg

	if e.initForAlb {
		InitAlbCr(e.Base, cfg)
	}
	InitCrds(e.Base, e.cfg, e.crds)
	if err != nil {
		return nil, err
	}
	raw, _ := KubeConfigFromREST(cfg, "test")
	kubeCfgPath := path.Join(e.Base, "default-kubecfg")
	os.WriteFile(kubeCfgPath, raw, 0666)
	e.cfgpath = kubeCfgPath
	return cfg, nil
}

func (e *EnvtestExt) AssertStart() *rest.Config {
	cfg, err := e.Start()
	if err != nil {
		panic(err)
	}
	return cfg
}

func (e *EnvtestExt) Stop() error {
	err := e.env.Stop()
	if err != nil {
		return err
	}
	return nil
}
