package workload

import (
	"alauda.io/alb2/pkg/operator/controllers/depl/resources"
	"alauda.io/alb2/pkg/operator/toolkit"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Option func(deploy *appv1.Deployment)

func setSelector(baseDomain, name, version string) Option {

	// TODO 在我们实现某种形式的deployment更新之前，不能动这里的label,否则会因为label不一致导致更新失败
	labels := map[string]string{
		"service_name":                    "alb2-" + name,
		"service." + baseDomain + "/name": toolkit.FmtKeyBySep("-", "deployment", name),
	}
	selector := &metav1.LabelSelector{MatchLabels: labels}
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		deploy.Spec.Selector = selector
	}
}

func setPodLabel(baseDomain, name string, version string) Option {
	labels := map[string]string{
		baseDomain + "/product":            "Platform-Center",
		"alb2." + baseDomain + "/pod_type": "alb",
	}
	commonlabel := map[string]string{
		"service_name":                    "alb2-" + name,
		"service." + baseDomain + "/name": toolkit.FmtKeyBySep("-", "deployment", name),
		"alb2." + baseDomain + "/version": version,
	}
	labels = resources.MergeLabel(labels, commonlabel)
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		deploy.Spec.Template.Labels = labels
	}
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

func WithHostNetwork(hostNetwork bool) Option {
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		if hostNetwork {
			deploy.Spec.Template.Spec.HostNetwork = true
			deploy.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
		} else {
			deploy.Spec.Template.Spec.HostNetwork = false
			deploy.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirst
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

func SetLabel(labels map[string]string) Option {
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		if deploy.Labels == nil {
			deploy.Labels = map[string]string{}
		}
		for k, v := range labels {
			deploy.Labels[k] = v
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

func SetAffinity(affinityKey, networkMode, labelBaseDomain string) Option {
	return func(deploy *appv1.Deployment) {
		if deploy == nil {
			return
		}
		var affinity *corev1.Affinity
		matchLabel := map[string]string{
			"alb2." + labelBaseDomain + "/type": affinityKey,
		}
		topologKey := "kubernetes.io/hostname"

		if networkMode == "host" {
			affinity = &corev1.Affinity{
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
		} else {
			affinity = &corev1.Affinity{
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

		deploy.Spec.Template.Spec.Affinity = affinity
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
