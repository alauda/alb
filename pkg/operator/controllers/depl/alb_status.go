package depl

import (
	"fmt"
	"reflect"
	"time"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	cfg "alauda.io/alb2/pkg/operator/config"
	patch "alauda.io/alb2/pkg/operator/controllers/depl/patch"
	. "alauda.io/alb2/utils"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"

	. "alauda.io/alb2/pkg/operator/toolkit"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// reconcile alb 中 alb的status的相关的逻辑

func GenExpectDeployStatus(deploy *appsv1.Deployment) albv2.DeployStatus {
	if deploy == nil {
		return albv2.DeployStatus{
			State:        albv2.ALB2StatePending,
			Reason:       "wait workload creating",
			ProbeTimeStr: metav1.Time{Time: time.Now()},
		}
	}
	ready, reason := deploymentReady(deploy)
	if !ready {
		return albv2.DeployStatus{
			State:        albv2.ALB2StateProgressing,
			Reason:       fmt.Sprintf("wait workload ready %v", reason),
			ProbeTimeStr: metav1.Time{Time: time.Now()},
		}
	}
	return albv2.DeployStatus{
		State:        albv2.ALB2StateRunning,
		Reason:       "workload ready",
		ProbeTimeStr: metav1.Time{Time: time.Now()},
	}
}

func deploymentReady(depl *appsv1.Deployment) (bool, string) {
	status := depl.Status
	spec := depl.Spec
	statusReplicas := status.Replicas
	statusReadyReplicas := status.ReadyReplicas
	specReplicas := status.Replicas
	if spec.Replicas != nil {
		specReplicas = *spec.Replicas
	}
	return statusReadyReplicas == statusReplicas && statusReadyReplicas == specReplicas, fmt.Sprintf("spec %v status %v %v", specReplicas, statusReplicas, statusReadyReplicas)
}

func GenExpectStatus(cf cfg.Config, cur *AlbDeploy) albv2.ALB2Status {
	versionStatus := albv2.VersionStatus{}
	conf := cf.ALB
	operator := cf.Operator
	hasPatch, alb, nginx := patch.GenImagePatch(&conf, operator)
	originStatus := cur.Alb.Status
	deploy := cur.Deploy
	versionStatus.Version = operator.Version
	if hasPatch {
		patchStatus := fmt.Sprintf("patched,alb: %v,nginx: %v", alb, nginx)
		versionStatus.ImagePatch = patchStatus
	} else {
		versionStatus.ImagePatch = "not patch"
	}

	status := originStatus.DeepCopy()
	status.Detail.Deploy = GenExpectDeployStatus(deploy)
	status.Detail.Versions = versionStatus
	status.Detail.AddressStatus = GenAddressStatusFromSvc(cur.Svc.LbSvc, cf)
	MergeAlbStatus(status, cf)
	return *status
}

func SameStatus(old, new albv2.ALB2Status, log logr.Logger) bool {
	old.ProbeTime = 0
	old.ProbeTimeStr = metav1.Time{}
	for i, p := range old.Detail.Alb.PortStatus {
		p.ProbeTimeStr = metav1.Time{}
		old.Detail.Alb.PortStatus[i] = p
	}
	old.Detail.Deploy.ProbeTimeStr = metav1.Time{}

	new.ProbeTime = 0
	new.ProbeTimeStr = metav1.Time{}
	for i, p := range new.Detail.Alb.PortStatus {
		p.ProbeTimeStr = metav1.Time{}
		new.Detail.Alb.PortStatus[i] = p
	}
	new.Detail.Deploy.ProbeTimeStr = metav1.Time{}
	log.Info("check status change", "diff", cmp.Diff(old, new))
	return reflect.DeepEqual(old, new)
}

func GenAddressStatusFromSvc(svc *corev1.Service, cf cfg.Config) albv2.AssignedAddress {
	// 如果没有开启lbsvc,则address的status是ok的
	ok := !cf.ALB.Vip.EnableLbSvc
	if IsNil(svc) {
		return albv2.AssignedAddress{Ok: ok}
	}
	v4 := []string{}
	v6 := []string{}
	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		if IsValidIPv4(ingress.IP) {
			v4 = append(v4, ingress.IP)
		}
		if IsValidIPv6(ingress.IP) {
			v6 = append(v6, ingress.IP)
		}
	}
	ok = (len(v4)+len(v6)) != 0 && cf.ALB.Vip.EnableLbSvc
	return albv2.AssignedAddress{
		Ok:   ok,
		Ipv4: v4,
		Ipv6: v6,
	}
}

func MergeAlbStatus(status *albv2.ALB2Status, cf cfg.Config) {
	// 1. deployment状态为ready
	// 2. alb的端口没有冲突
	// 3. 开启lbsvc时alb 分配到了地址

	status.ProbeTimeStr = metav1.Time{Time: time.Now()}
	status.ProbeTime = time.Now().Unix()
	{
		if status.Detail.Deploy.State != albv2.ALB2StateRunning {
			status.State = status.Detail.Deploy.State
			status.Reason = status.Detail.Deploy.Reason
			return
		}
	}
	// port conflict
	{
		if len(status.Detail.Alb.PortStatus) != 0 {
			status.State = albv2.ALB2StateWarning
			reason := ""
			for port, msg := range status.Detail.Alb.PortStatus {
				reason += fmt.Sprintf("%s %s.", port, msg.Msg)
			}
			status.Reason = reason
			return
		}
	}
	// lb svc not ready
	{
		if cf.ALB.Vip.EnableLbSvc && !status.Detail.AddressStatus.Ok {
			status.State = albv2.ALB2StateProgressing
			status.Reason = "wait lb svc ready"
			return
		}
	}
	status.State = albv2.ALB2StateRunning
	status.Reason = ""
}
