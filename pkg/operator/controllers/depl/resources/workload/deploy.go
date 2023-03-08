package workload

import (
	"alauda.io/alb2/pkg/operator/config"
	"alauda.io/alb2/pkg/operator/toolkit"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	defaultMaxSurge       = intstr.FromInt(0)
	defaultMaxUnavailable = intstr.FromInt(1)
	defaultReplicas       = int32(1)
)

type Template struct {
	namespace  string
	name       string
	baseDomain string
	env        config.OperatorCfg
	albcfg     *config.ALB2Config
	cur        *appv1.Deployment
}

func NewTemplate(namespace string, name string, baseDomain string, cur *appv1.Deployment, albcf *config.ALB2Config, cfg config.OperatorCfg) *Template {
	return &Template{
		namespace:  namespace,
		name:       name,
		baseDomain: baseDomain,
		env:        cfg,
		cur:        cur,
		albcfg:     albcf,
	}
}

type VolumeCfg struct {
	Volumes map[string]corev1.Volume
	Mounts  map[string]struct {
		name string
		path string
	}
}

func (b *Template) Generate(options ...Option) *appv1.Deployment {
	deploy := b.generate()
	cmVolume := b.configmapVolume(b.name)
	sVolume := b.shareVolume()
	defaultOptions := []Option{
		setPodLabel(b.baseDomain, b.name, b.env.Version, b.albcfg.Deploy.AntiAffinityKey),
		setSelector(b.baseDomain, b.name, b.env.Version),
		AddVolumeMount(sVolume, "/etc/alb2/nginx/"),
		AddVolumeMount(cmVolume, "/alb/tweak/"),
	}
	for _, op := range defaultOptions {
		op(deploy)
	}
	for _, op := range options {
		op(deploy)
	}
	return deploy
}

func (b *Template) generate() *appv1.Deployment {
	nginxConatiner := b.nginxContainer()
	albConatiner := b.albContainer()
	gTrue := true

	deployment := &appv1.Deployment{}
	if !toolkit.IsNil(b.cur) {
		deployment = b.cur
	}
	deployment.TypeMeta = v1.TypeMeta{
		Kind:       "Deployment",
		APIVersion: "apps/v1",
	}
	deployment.ObjectMeta = v1.ObjectMeta{
		Name:      b.name,
		Namespace: b.namespace,
	}
	deployment.Spec = appv1.DeploymentSpec{
		Replicas: &defaultReplicas,
		Strategy: appv1.DeploymentStrategy{
			Type: appv1.RollingUpdateDeploymentStrategyType,
			RollingUpdate: &appv1.RollingUpdateDeployment{
				MaxSurge:       &defaultMaxSurge,
				MaxUnavailable: &defaultMaxUnavailable,
			},
		},

		Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Tolerations: []corev1.Toleration{
					{
						Operator: corev1.TolerationOperator("Exists"),
					},
				},
				ShareProcessNamespace: &gTrue,
				Containers: []corev1.Container{
					nginxConatiner,
					albConatiner,
				},
			},
		},
	}
	return deployment
}

func (b *Template) nginxContainer() corev1.Container {
	img := b.env.NginxImage
	return corev1.Container{
		Name:  "nginx",
		Image: img,
		Command: []string{
			"/alb/nginx/run-nginx.sh",
		},
		ImagePullPolicy: "Always",
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"SYS_PTRACE",
					"NET_BIND_SERVICE",
				},
			},
		},
	}
}

func (b *Template) albContainer() corev1.Container {
	img := b.env.AlbImage
	return corev1.Container{
		Name:  "alb2",
		Image: img,
		Command: []string{
			"/alb/ctl/run-alb.sh",
		},
		ImagePullPolicy: "Always",
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"SYS_PTRACE",
				},
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("2Gi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
	}
}

func (b *Template) configmapVolume(cmName string) corev1.Volume {
	return corev1.Volume{
		Name: "tweak-conf",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cmName,
				},
				Items: []corev1.KeyToPath{
					{
						Key:  "http",
						Path: "http.conf",
					},
					{
						Key:  "http_server",
						Path: "http_server.conf",
					},
					{
						Key:  "grpc_server",
						Path: "grpc_server.conf",
					},
					{
						Key:  "upstream",
						Path: "upstream.conf",
					},
					{
						Key:  "stream-tcp",
						Path: "stream-tcp.conf",
					},
					{
						Key:  "stream-udp",
						Path: "stream-udp.conf",
					},
					{
						Key:  "stream-common",
						Path: "stream-common.conf",
					},
					{
						Key:  "bind_nic",
						Path: "bind_nic.json",
					},
				},
			},
		},
	}
}

func (b *Template) shareVolume() corev1.Volume {
	return corev1.Volume{
		Name: "share-conf",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}
