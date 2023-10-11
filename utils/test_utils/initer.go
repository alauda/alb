package test_utils

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path"
	"runtime"

	"alauda.io/alb2/utils/log"
	"k8s.io/client-go/rest"
)

func AlbCrds() []string {
	crds := []string{}
	crds = append(crds, GetAlbBase()+"/deploy/chart/alb/crds")
	crds = append(crds, GetAlbBase()+"/scripts/yaml/crds/extra/v1")

	return crds
}

func InitAlbNs(albBase string, cfg *rest.Config) error {
	kc := NewK8sClient(context.Background(), cfg)
	return kc.CreateNsIfNotExist("cpaas-system")
}

func InitCrds(base string, cfg *rest.Config, crds []string) error {
	k := NewKubectl(base, cfg, log.L())
	for _, crd := range crds {
		cmds := []string{"apply", "-f", crd, "-R"}
		_, err := k.Kubectl(cmds...)
		if err != nil {
			return err
		}
	}
	return nil
}

// install alb related crds include alb's crd and 'features'
// create cpaas-system ns
func InitAlbCr(base string, cfg *rest.Config) error {
	InitAlbNs(base, cfg)
	InitCrds(base, cfg, AlbCrds())
	return nil
}

func BaseWithDir(base, subdir string) string {
	p := path.Join(base, subdir)
	os.MkdirAll(p, os.ModePerm)
	return p
}

func BaseWithRandomDir(base, subdirprefix string) string {
	return BaseWithDir(base, fmt.Sprintf("%s-%v", subdirprefix, rand.Int()))
}

func GetAlbBase() string {
	_, filename, _, _ := runtime.Caller(0)
	albBase := path.Join(path.Dir(filename), "../../")
	return albBase
}

func InitBase() string {
	name := "alb-test-base"
	var base string
	var err error
	if os.Getenv("ALB_TEST_BASE") != "" {
		base, err = os.MkdirTemp(os.Getenv("ALB_TEST_BASE"), "alb-e2e-test")
		if err != nil {
			panic(err)
		}
		return base
	}
	if os.Getenv("DEV_MODE") == "true" {
		base = path.Join(os.TempDir(), name)
	} else {
		base, err = os.MkdirTemp("", "alb-e2e-test")
		if err != nil {
			panic(err)
		}
	}
	if err := os.RemoveAll(base); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(base, os.ModePerm); err != nil {
		panic(err)
	}
	return base
}
