package test_utils

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

type Kubectl struct {
	kubeCfgPath string
	base        string
}

// if base =="" it will create /tmp/kubectl-xx else create base/kubectl
func NewKubectl(base string, kubeCfg *rest.Config) *Kubectl {
	if base == "" {
		base = path.Join(os.TempDir(), fmt.Sprintf("kubectl-%d", rand.Int()))
		os.Mkdir(base, 0777)
	} else {
		base = path.Join(base, "kubectl")
	}
	os.Mkdir(base, 0777)
	raw, _ := KubeConfigFromREST(kubeCfg, "test")
	kubeCfgPath := path.Join(base, "kubecfg")
	os.WriteFile(kubeCfgPath, raw, 0666)
	return &Kubectl{
		base:        base,
		kubeCfgPath: kubeCfgPath,
	}
}

func (k *Kubectl) KubectlApply(yaml string, options ...string) (string, error) {
	p, err := RandomFile(k.base, yaml)
	if err != nil {
		return "", err
	}
	cmds := []string{"apply", "-f", p}
	cmds = append(cmds, options...)
	Logf("cmds: kubectl %v", strings.Join(cmds, " "))
	return k.Kubectl(cmds...)
}

func (k *Kubectl) AssertKubectlApply(yaml string, options ...string) string {
	ret, err := k.KubectlApply(yaml, options...)
	assert.Nil(ginkgo.GinkgoT(), err, "")
	return ret
}

func (k *Kubectl) AssertKubectlApplyFile(p string, options ...string) string {
	cmds := []string{"apply", "-f", p}
	cmds = append(cmds, options...)
	Logf("cmds: kubectl %v", strings.Join(cmds, " "))
	return k.AssertKubectl(cmds...)
}

func (k *Kubectl) Kubectl(cmds ...string) (string, error) {
	if len(cmds) == 1 {
		cmds = strings.Split(cmds[0], " ")
	}
	cmds = append(cmds, "--kubeconfig", k.kubeCfgPath)
	fmt.Printf("cmds %v", cmds)
	cmd := exec.Command("kubectl", cmds...)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s err: %v", stdout, err)
	}
	return string(stdout), nil
}

func (k *Kubectl) AssertKubectl(cmds ...string) string {
	ret, err := k.Kubectl(cmds...)
	assert.Nil(ginkgo.GinkgoT(), err, "")
	return ret
}

func (k *Kubectl) CleanUp() error {
	return os.RemoveAll(k.base)
}
