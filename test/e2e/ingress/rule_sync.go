package ingress

import (
	"fmt"
	"os"
	"path"
	"runtime"

	"alauda.io/alb2/test/e2e/framework"
	. "github.com/onsi/ginkgo"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func cfd() string {
	_, filename, _, _ := runtime.Caller(0)
	return path.Dir(filename)
}

var tlog = framework.Logf

var _ = Describe("Ingress", func() {
	var f *framework.Framework

	framework.GIt("test ingress startup sync", func() {
		os.Setenv("ALB_RELOAD_NGINX", "false")
		os.Setenv("ALB_ENABLE_GATEWAY", "false")
		os.Setenv("ALB_LEADER_LEASE_DURATION", "3000")
		os.Setenv("ALB_LEADER_RENEW_DEADLINE", "2000")
		os.Setenv("ALB_LEADER_RETRY_PERIOD", "1000")
		os.Setenv("DEFAULT-SSL-CERTIFICATE", "cpaas-system/default")
		os.Setenv("DEFAULT-SSL-STRATEGY", "Always")
		deployCfg := framework.Config{InstanceMode: true, RestCfg: framework.CfgFromEnv(), Project: []string{"ALL_ALL"}}
		f = framework.NewAlb(deployCfg)
		f.Init()
		defer f.Destroy()

		expectRuleNum := 81
		f.AssertKubectlApplyFile(path.Join(cfd(), "./all.ingress"))
		f.Wait(func() (bool, error) {
			// 检查rule数量
			rules, err := f.GetAlbClient().CrdV1().Rules(f.GetNamespace()).List(f.GetCtx(), v1.ListOptions{})
			if err != nil {
				return false, err
			}
			httpsftid := ""
			for _, r := range rules.Items {
				ft := r.OwnerReferences[0]
				tlog("rule ft id %v %v", ft.UID, ft.Name)
				if ft.Name == "alb-dev-00443" && httpsftid == "" {
					httpsftid = string(ft.UID)
				}
				if httpsftid != string(ft.UID) {
					return false, fmt.Errorf("invalid ft id")
				}
			}
			tlog("rule len %v", len(rules.Items))
			if len(rules.Items) == expectRuleNum {
				return true, nil
			}
			return false, nil
		})
		tlog("restart alb,it should not recreate rule")
		f.RestartAlb()
		expectWaitCount := 5
		waitCount := 0
		f.Wait(func() (bool, error) {
			// 检查rule数量
			rules, err := f.GetAlbClient().CrdV1().Rules(f.GetNamespace()).List(f.GetCtx(), v1.ListOptions{})
			if err != nil {
				return false, err
			}
			tlog("rule len %v", len(rules.Items))
			if len(rules.Items) == expectRuleNum {
				waitCount++
			}
			return waitCount == expectWaitCount, nil
		})
	})

	// TODO ingress创建出的ft,修改了ft的默认证书,当重启时,ft的默认证书不变.
	framework.GIt("should keep ft default cert when restart", func() {
	})

	// TODO 删除了ingress,并且这个ingress是有默认路由的,应该把ft的默认后端删除
	framework.GIt("should keep ft default cert when restart", func() {
	})
})
