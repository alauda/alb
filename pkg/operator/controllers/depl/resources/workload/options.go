package workload

import (
	a2t "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	. "alauda.io/alb2/pkg/operator/controllers/depl/util"
	"alauda.io/alb2/pkg/operator/toolkit"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Option func(deploy *appv1.Deployment)

func setSelector(labels map[string]string) Option {
	selector := &metav1.LabelSelector{MatchLabels: labels}
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		deploy.Spec.Selector = selector
	}
}

func (d *DeplTemplate) getAntiaffinitykey() string {
	return d.albcfg.Deploy.AntiAffinityKey
}

func (d *DeplTemplate) podSelector() map[string]string {
	// TODO 在我们实现某种形式的deployment更新之前，不能动这里的label,否则会因为label不一致导致更新失败
	baseDomain := d.baseDomain
	name := d.name
	labels := map[string]string{
		"service_name":                    "alb2-" + name,
		"service." + baseDomain + "/name": toolkit.FmtKeyBySep("-", "deployment", name),
	}
	return labels
}

func (d *DeplTemplate) expectPodLabel() map[string]string {
	baseDomain := d.baseDomain
	name := d.name
	antiaffinitykey := d.getAntiaffinitykey()
	version := d.env.Version

	labels := map[string]string{
		baseDomain + "/product":            "Platform-Center",
		"alb2." + baseDomain + "/pod_type": "alb",
	}
	commonlabel := map[string]string{
		"service_name":                    "alb2-" + name,
		"service." + baseDomain + "/name": toolkit.FmtKeyBySep("-", "deployment", name),
		"alb2." + baseDomain + "/version": version,
	}
	if d.albcfg.Controller.NetworkMode == a2t.HOST_MODE {
		commonlabel["alb2."+baseDomain+"/type"] = antiaffinitykey
	}
	labels = MergeMap(labels, commonlabel)
	return labels
}

func SetImage(name, image string) Option {
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		containers := deploy.Spec.Template.Spec.Containers
		for index := range containers {
			if containers[index].Name == name {
				containers[index].Image = image
			}
		}
	}
}

func SetALB2Image(alb, nginx string) Option {
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		containers := deploy.Spec.Template.Spec.Containers
		for index := range containers {
			if containers[index].Name == "alb2" {
				containers[index].Image = alb
			}
			if containers[index].Name == "nginx" {
				containers[index].Image = nginx
			}
		}
	}
}

func SetNodeSelector(nodeSelector map[string]string) Option {
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		if deploy.Spec.Template.Spec.Volumes == nil {
			deploy.Spec.Template.Spec.Volumes = []corev1.Volume{}
		}
		deploy.Spec.Template.Spec.NodeSelector = nodeSelector
	}
}

func AddVolumeMount(volume corev1.Volume, dstDir string) Option {
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		if deploy.Spec.Template.Spec.Volumes == nil {
			deploy.Spec.Template.Spec.Volumes = []corev1.Volume{}
		}
		deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, volume)

		containers := deploy.Spec.Template.Spec.Containers
		for index := range containers {
			if containers[index].VolumeMounts == nil {
				containers[index].VolumeMounts = []corev1.VolumeMount{}
			}
			containers[index].VolumeMounts = append(containers[index].VolumeMounts,
				corev1.VolumeMount{
					Name:      volume.Name,
					MountPath: dstDir,
				},
			)
		}
	}
}

func SetLivenessProbe(name string, probe *corev1.Probe) Option {
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		containers := deploy.Spec.Template.Spec.Containers
		for index := range containers {
			if containers[index].Name == name {
				containers[index].LivenessProbe = probe
			}
		}
	}
}

func AddEnv(newEnv corev1.EnvVar, containerName string) Option {
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		containers := deploy.Spec.Template.Spec.Containers
		for index := range deploy.Spec.Template.Spec.Containers {
			if containers[index].Name == containerName {
				override := false
				for i, env := range containers[index].Env {
					if env.Name == newEnv.Name {
						containers[index].Env[i] = newEnv
						override = true
					}
				}
				if !override {
					containers[index].Env = append(containers[index].Env, newEnv)
				}
				break
			}
		}
	}
}

func SetEnvs(envs []corev1.EnvVar, containerName string) Option {
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		containers := deploy.Spec.Template.Spec.Containers
		for index := range containers {
			if containers[index].Name == containerName {
				containers[index].Env = envs
				break
			}

		}
	}
}

func SetOwnerRefs(reference []metav1.OwnerReference) Option {
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		deploy.OwnerReferences = reference
	}
}

func SetReplicas(replicas int32) Option {
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		deploy.Spec.Replicas = &replicas
	}
}

func (d *DeplTemplate) GenExpectAffinity() *corev1.Affinity {
	labelBaseDomain := d.env.BaseDomain
	networkMode := d.albcfg.Controller.NetworkMode
	affinityKey := d.albcfg.Deploy.AntiAffinityKey
	matchLabel := map[string]string{
		"alb2." + labelBaseDomain + "/type": affinityKey,
	}
	topologKey := "kubernetes.io/hostname"

	if networkMode == a2t.HOST_MODE {
		return &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: matchLabel,
						},
						TopologyKey: topologKey,
					},
				},
			},
		}
	}
	// 容器网络模式的alb可以调度在任意节点上
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: matchLabel,
						},
						TopologyKey: topologKey,
					},
				},
			},
		},
	}
}

func SetResource(resource corev1.ResourceRequirements, containerName string) Option {
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		containers := deploy.Spec.Template.Spec.Containers
		for index := range containers {
			if containers[index].Name == containerName {
				containers[index].Resources = resource
				break
			}

		}
	}
}
