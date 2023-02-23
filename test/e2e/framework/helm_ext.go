package framework

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"alauda.io/alb2/utils/test_utils"
	. "github.com/onsi/ginkgo"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

type Helm struct {
	helmBase    string
	kubeCfgPath string
}

func NewHelm(base string, kubeCfg *rest.Config) *Helm {
	raw, _ := test_utils.KubeConfigFromREST(kubeCfg, "test")
	helmBase := path.Join(base, "helm")
	_ = os.Mkdir(helmBase, os.ModePerm)
	kubeCfgPath := path.Join(helmBase, "kubecfg")
	os.WriteFile(kubeCfgPath, raw, 0600)
	return &Helm{
		helmBase:    helmBase,
		kubeCfgPath: kubeCfgPath,
	}
	// helm ignore kubecfg permission
}

func (h *Helm) Install(cfgs []string, name string, base, val string) (string, error) {
	releaseBase := path.Join(h.helmBase, name)
	os.MkdirAll(releaseBase, os.ModePerm)
	for index, yaml := range cfgs {
		os.WriteFile(path.Join(releaseBase, fmt.Sprintf("ov.%d.yaml", index)), []byte(yaml), 0666)
	}
	cmds := []string{"install", "-f", val}
	for index := range cfgs {
		cmds = append(cmds, "-f")
		cmds = append(cmds, path.Join(releaseBase, fmt.Sprintf("ov.%d.yaml", index)))
	}
	cmds = append(cmds, name, base)
	cmds = append(cmds, "--kubeconfig", h.kubeCfgPath, "--create-namespace")
	return helm(cmds...)
}

func (h *Helm) UnInstall(name string) (string, error) {
	cmds := []string{"uninstall", name, "--kubeconfig", h.kubeCfgPath}
	return helm(cmds...)
}

func (h *Helm) List() ([]string, error) {
	cmds := []string{"list", "--kubeconfig", h.kubeCfgPath}
	out, err := helm(cmds...)
	if err != nil {
		return nil, err
	}
	return lo.FilterMap(strings.Split(out, "\n")[1:], func(l string, _ int) (string, bool) {
		if len(l) == 0 {
			return "", false
		}
		name := strings.Fields(l)[0]
		return name, true
	}), nil
}

func (h *Helm) UnInstallAll() (string, error) {
	list, err := h.List()
	if err != nil {
		return "", err
	}
	for _, name := range list {
		_, err := h.UnInstall(name)
		if err != nil {
			return "", err
		}
	}
	return "", nil
}

func (h *Helm) AssertUnInstall(name string) string {
	out, err := h.UnInstall(name)
	assert.NoError(GinkgoT(), err)
	return out
}

func (h *Helm) AssertUnInstallAll() string {
	out, err := h.UnInstallAll()
	assert.NoError(GinkgoT(), err)
	return out
}

func (h *Helm) AssertInstall(cfgs []string, name string, base, val string) string {
	out, err := h.Install(cfgs, name, base, val)
	assert.NoError(GinkgoT(), err, "helm install fail")
	return out
}

func helm(cmds ...string) (string, error) {
	Logf("helm %v", cmds)
	cmd := exec.Command("helm", cmds...)
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s err: %v", stdout, err)
	}
	return string(stdout), nil
}
