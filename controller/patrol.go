package controller

import (
	. "alauda.io/alb2/controller/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apit "k8s.io/apimachinery/pkg/types"
)

// 有些对alb的巡检,在每次生成的时候顺便做了
func (nc *NginxController) patrol(lb *LoadBalancer) {
	// 每个alb的pod都会检测节点上的端口是否冲突了
	if nc.PortProber != nil {
		nc.PortProber.WorkerDetectAndMaskConflictPort(lb)
	}
	if nc.lc != nil && nc.lc.AmILeader() {
		nc.migratePortProject(lb)
	}
}

func (nc *NginxController) migratePortProject(alb *LoadBalancer) {
	driver := nc.Driver
	ctx := nc.Ctx
	domain := nc.albcfg.GetDomain()
	var portInfo map[string][]string
	if GetAlbRoleType(alb.Labels, domain) != RolePort {
		return
	}
	portInfo, err := getPortInfo(driver, nc.albcfg.Ns, nc.albcfg.Name)
	if err != nil {
		nc.log.Error(err, "get port project info failed")
		return
	}
	for _, ft := range alb.Frontends {
		if GetAlbRoleType(alb.Labels, domain) == RolePort && portInfo != nil {
			// current projects
			portProjects := GetOwnProjectsFromLabel(ft.FtName, ft.Labels, nc.albcfg.GetDomain())
			// desired projects
			desiredPortProjects, err := getPortProject(int(ft.Port), portInfo)
			if err != nil {
				nc.log.Error(err, "get port desired projects failed", "port", ft.Port)
				return
			}
			if !SameProject(portProjects, desiredPortProjects) {
				// diff need update
				payload := generatePatchPortProjectPayload(ft.Labels, desiredPortProjects, nc.albcfg.GetDomain())
				nc.log.Info("update ft project ", "payload", string(payload))
				if _, err := driver.ALBClient.CrdV1().Frontends(nc.albcfg.GetNs()).Patch(ctx, ft.FtName, apit.JSONPatchType, payload, metav1.PatchOptions{}); err != nil {
					nc.log.Error(err, "patch port project failed", "ft", ft.FtName)
				}
			}
		}
	}
}

func SameProject(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftMap := make(map[string]bool)
	for _, p := range left {
		leftMap[p] = true
	}
	for _, p := range right {
		ok, find := leftMap[p]
		if !ok || !find {
			return false
		}
	}
	return true
}
