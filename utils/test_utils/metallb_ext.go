package test_utils

import (
	"context"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
)

type MetallbInstaller struct {
	cfg    *rest.Config
	log    logr.Logger
	ctx    context.Context
	kind   *KindExt   // we need load image
	docker *DockerExt // we need get docker subnet
	v4pool mapset.Set[string]
	v6pool mapset.Set[string]
	kt     *Kubectl
}

func NewMetallbInstaller(ctx context.Context, base string, cfg *rest.Config, kind *KindExt, v4 []string, v6 []string, log logr.Logger) *MetallbInstaller {
	kt := NewKubectl(base, cfg, log)
	docker := NewDockerExt(log)
	return &MetallbInstaller{
		cfg:    cfg,
		log:    log,
		ctx:    ctx,
		kind:   kind,
		docker: &docker,
		v4pool: mapset.NewSet(v4...),
		v6pool: mapset.NewSet(v6...),
		kt:     kt,
	}
}

func (m *MetallbInstaller) Init() error {
	ok, err := m.HasInStall()
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	err = m.InStall()
	if err != nil {
		return err
	}
	return nil
}

func (m *MetallbInstaller) HasInStall() (bool, error) {
	out, err := m.kt.Kubectl("get po -n metallb-system")
	if err != nil {
		return false, nil
	}
	return strings.Contains(out, "controller"), nil
}

func (m *MetallbInstaller) InStall() error {
	m.log.Info("install metallb")
	m.kind.LoadImage("quay.io/metallb/speaker:v0.13.7")
	m.kind.LoadImage("quay.io/metallb/controller:v0.13.7")
	//  https://raw.githubusercontent.com/metallb/metallb/v0.13.7/config/manifests/metallb-native.yaml
	url := "http://prod-minio.alauda.cn:80/acp/metallb-native-0.13.7.yaml"
	out, err := m.kt.Kubectl("apply -f " + url)
	if err != nil {
		return err
	}
	m.kt.AssertKubectlApply(`
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: example
  namespace: metallb-system
spec:
  addresses:
  - 172.18.253.1-172.18.253.100
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: empty
  namespace: metallb-system
`)
	m.log.Info("init metallb ", "out", out)
	return nil
}
