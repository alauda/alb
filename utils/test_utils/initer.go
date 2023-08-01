package test_utils

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path"

	"alauda.io/alb2/utils/log"
	"k8s.io/client-go/rest"
)

func InitCrd(albBase string, cfg *rest.Config) error {
	k := NewKubectl("", cfg, log.L())
	kc := NewK8sClient(context.Background(), cfg)
	kc.CreateNsIfNotExist("cpaas-system")
	{
		// init crd
		crd := path.Join(albBase, "deploy", "chart", "alb", "crds")
		cmds := []string{"apply", "-f", crd, "-R"}
		_, err := k.Kubectl(cmds...)
		if err != nil {
			return err
		}
	}
	{
		cmds := []string{"apply", "-f", path.Join(albBase, "./scripts/yaml/crds/extra/v1"), "-R"}
		_, err := k.Kubectl(cmds...)
		if err != nil {
			return err
		}
	}
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
