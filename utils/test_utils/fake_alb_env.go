package test_utils

import (
	"context"
	"fmt"

	"alauda.io/alb2/utils/log"
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FakeAlbEnv is a env which is could apply FakeResource
type FakeAlbEnv struct {
	ctx  context.Context
	e    *EnvtestExt
	base string
	l    logr.Logger
	kc   *K8sClient
}

func NewFakeEnv() FakeAlbEnv {
	return FakeAlbEnv{
		ctx:  context.Background(),
		l:    log.L(),
		base: InitBase(),
	}
}

func (a *FakeAlbEnv) GetCfg() *rest.Config {
	return a.e.cfg
}

func (a *FakeAlbEnv) AssertStart() {
	a.e = NewEnvtestExt(a.base, a.l)
	a.e.AssertStart()

	a.kc = NewK8sClient(a.ctx, a.e.cfg)
}

func (a *FakeAlbEnv) ApplyFakes(fake FakeResource) error {
	for _, res := range fake.ListCr() {
		err := a.kc.ctlClient.Create(a.ctx, res)
		fmt.Printf("create cr %+v %s %s\n", PrettyCr(res), res.GetNamespace(), res.GetName())
		if err != nil {
			return fmt.Errorf("crate cr %+v fail %v", res, err)
		}
	}
	return nil
}

func (a *FakeAlbEnv) Stop() {
	a.e.Stop()
}

func ignoreMethodNotAllowed(err error) error {
	if err != nil {
		if apierrors.ReasonForError(err) == metav1.StatusReasonMethodNotAllowed {
			return nil
		}
	}
	return err
}
