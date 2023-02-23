package framework

import (
	"path"
	"runtime"

	"alauda.io/alb2/utils/test_utils"
	"k8s.io/client-go/rest"
)

func AlbBeforeSuite(cfg *rest.Config) {
	_, filename, _, _ := runtime.Caller(0)
	albBase := path.Join(path.Dir(filename), "../../../")
	err := test_utils.InitCrd(albBase, cfg)
	GinkgoNoErr(err)
	_, err = InitKubeCfgEnv(cfg)
	GinkgoNoErr(err)
}

func AlbAfterSuite() {
	// TODO delete those tmep file
}
