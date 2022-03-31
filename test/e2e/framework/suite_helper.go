package framework

import (
	"path"
	"runtime"

	. "github.com/onsi/ginkgo"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

func AlbBeforeSuite(cfg *rest.Config) {
	p, err := InitKubeCfg(cfg)
	assert.NoError(GinkgoT(), err)
	_, filename, _, _ := runtime.Caller(0)
	dir := path.Join(path.Dir(filename), "../../../chart/crds")
	Logf("start up envtest %+v in kubecfg %v dir %v", cfg.Host, p, dir)
	// init crd
	cmds := []string{"apply", "-f", dir, "-R", "--kubeconfig", p}
	out, err := Kubectl(cmds...)
	Logf("init crd %v", out)
	assert.NoError(GinkgoT(), err)
}

func AlbAfterSuite() {

}
