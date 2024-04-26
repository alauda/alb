package test_utils

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

type Kubectl struct {
	kubeCfgPath string
	base        string
	log         logr.Logger
}

// if base =="" it will create /tmp/kubectl-xx else create base/kubectl-xx
func NewKubectl(base string, kubeCfg *rest.Config, log logr.Logger) *Kubectl {
	cfg := fmt.Sprintf("kubectl-%d", rand.Int())
	if base == "" {
		base = path.Join(os.TempDir(), cfg)
		os.Mkdir(base, 0o777)
	} else {
		base = path.Join(base, cfg)
	}
	os.Mkdir(base, 0o777)
	raw, _ := KubeConfigFromREST(kubeCfg, "test")
	kubeCfgPath := path.Join(base, cfg)
	os.WriteFile(kubeCfgPath, raw, 0o666)
	return &Kubectl{
		base:        base,
		kubeCfgPath: kubeCfgPath,
		log:         log,
	}
}

func (k *Kubectl) KubectlApply(yaml string, options ...string) (string, error) {
	p, err := RandomFile(k.base, yaml)
	if err != nil {
		return "", err
	}
	cmds := []string{"apply", "-f", p}
	cmds = append(cmds, options...)
	k.log.Info("kubectl", "cmd", strings.Join(cmds, " "))
	return k.Kubectl(cmds...)
}

func (k *Kubectl) AssertKubectlApply(yaml string, options ...string) string {
	ret, err := k.KubectlApply(yaml, options...)
	if err != nil {
		k.log.Error(err, "apply yaml fail", "yaml", yaml)
	}
	assert.Nil(ginkgo.GinkgoT(), err, "apply yaml fail")
	return ret
}

func (k *Kubectl) AssertKubectlApplyFile(p string, options ...string) string {
	cmds := []string{"apply", "-f", p}
	cmds = append(cmds, options...)
	k.log.Info("kubectl", "cmd", strings.Join(cmds, " "))
	return k.AssertKubectl(cmds...)
}

func (k *Kubectl) VerboseKubectl(cmds ...string) {
	out, err := k.Kubectl(cmds...)
	if err != nil {
		k.log.Error(err, "sth wrong")
		return
	}
	k.log.Info(out)
}

func (k *Kubectl) Kubectl(cmds ...string) (string, error) {
	if len(cmds) == 1 {
		cmds = strings.Split(cmds[0], " ")
	}
	cmds = append(cmds, "--kubeconfig", k.kubeCfgPath)
	k.log.Info("cmd", "cmds", cmds)
	cmd := exec.Command("kubectl", cmds...)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("eval %s %s err: %v", cmd, stdout, err)
	}
	return string(stdout), nil
}

func (k *Kubectl) AssertKubectl(cmds ...string) string {
	ret, err := k.Kubectl(cmds...)
	assert.Nil(ginkgo.GinkgoT(), err, "")
	return ret
}

func (k *Kubectl) AssertKubectlOmgea(o gomega.Gomega, cmds ...string) string {
	ret, err := k.Kubectl(cmds...)
	o.Expect(err).ToNot(gomega.HaveOccurred())
	return ret
}

func (k *Kubectl) GetKubecfg() string {
	return k.kubeCfgPath
}

func (k *Kubectl) CleanUp() error {
	return os.RemoveAll(k.base)
}
