package test_utils

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	"k8s.io/utils/strings/slices"

	"github.com/ztrue/tracerr"
)

type Helm struct {
	helmBase    string
	kubeCfgPath string
	Log         logr.Logger
}

// @require helm # which must be helm-alauda NOT helm v3
func NewHelm(base string, kubeCfg *rest.Config, l logr.Logger) *Helm {
	helmBase := BaseWithDir(base, "helm")
	kubeCfgPath := ""
	if kubeCfg != nil {
		raw, _ := KubeConfigFromREST(kubeCfg, "test")
		kubeCfgPath = path.Join(helmBase, "kubecfg")
		os.WriteFile(kubeCfgPath, raw, 0600)
	}

	return &Helm{
		helmBase:    helmBase,
		kubeCfgPath: kubeCfgPath,
		Log:         l,
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
	return h.helm(cmds...)
}

func (h *Helm) UnInstall(name string) (string, error) {
	charts, err := h.List()
	if err != nil {
		return "", err
	}
	if !slices.Contains(charts, name) {
		h.Log.Info("not found ignore", "name", name)
		return "", nil
	}
	cmds := []string{"uninstall", name, "--kubeconfig", h.kubeCfgPath}
	return h.helm(cmds...)
}

func (h *Helm) List() ([]string, error) {
	cmds := []string{"list", "--kubeconfig", h.kubeCfgPath}
	out, err := h.helm(cmds...)
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

func HelmPull(chart string, dir string, log logr.Logger) (string, error) {
	log.Info("helm pull", "chart", chart, "dir", dir)
	out, err := helmWithBase([]string{"chart", "pull", chart, "--insecure"}, "", log)
	if err != nil {
		return "", err
	}
	log.Info("helm", "msg", out)
	out, err = helmWithBase([]string{"chart", "export", chart}, dir, log)
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`Exported chart to ([^/]*)/`)
	matches := re.FindStringSubmatch(out)
	log.Info("export", "msg", out)
	log.Info("export", "match", matches)
	if len(matches) != 2 {
		return "", fmt.Errorf("export fail %s", out)
	}
	return path.Join(dir, matches[1]), nil
}

func (h *Helm) Pull(chart string) (string, error) {
	dir, err := os.MkdirTemp(h.helmBase, "chart*")
	if err != nil {
		return "", tracerr.Wrap(err)
	}
	return HelmPull(chart, dir, h.Log)
}

func helmWithBase(cmds []string, dir string, log logr.Logger) (string, error) {

	log.Info("helm call", "cmds", cmds, "dir", dir)

	cmd := exec.Command("helm", cmds...)
	if dir != "" {
		cmd.Dir = dir
	}
	stdout, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s err: %v", stdout, err)
	}
	return string(stdout), nil
}

func (h *Helm) helm(cmds ...string) (string, error) {
	return helmWithBase(cmds, "", h.Log)
}

func (h *Helm) Destory() error {
	return os.RemoveAll(h.helmBase)
}
