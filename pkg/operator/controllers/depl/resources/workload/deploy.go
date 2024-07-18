package workload

import (
	"fmt"
	"reflect"

	a2t "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/pkg/operator/config"
	"alauda.io/alb2/pkg/operator/controllers/depl/patch"
	. "alauda.io/alb2/pkg/operator/controllers/depl/resources/types"
	. "alauda.io/alb2/pkg/operator/controllers/depl/util"
	. "alauda.io/alb2/pkg/operator/toolkit"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	pointer "k8s.io/utils/ptr"
)

var (
	defaultMaxSurge       = intstr.FromInt(0)
	defaultMaxUnavailable = intstr.FromInt(1)
)

const (
	ALB_CONTAINER_NAME   = "alb2"
	NGINX_CONTAINER_NAME = "nginx"
)

type DeplTemplate struct {
	alb        *a2t.ALB2
	name       string
	ns         string
	baseDomain string
	env        config.OperatorCfg
	albcfg     *config.ALB2Config
	cur        *appv1.Deployment
	log        logr.Logger
}

// what we cared in deployment
type DeployCfg struct {
	Alb   DeployContainerCfg
	Nginx DeployContainerCfg
	Spec  DeploySpec
	Vol   VolumeCfg
}

type DeploySpec struct {
	Name           string
	Ns             string
	DeployLabel    map[string]string
	DeplAnnotation map[string]string
	PodLabel       map[string]string
	Replica        *int32
	Owner          []v1.OwnerReference
	Hostnetwork    bool
	Dnspolicy      corev1.DNSPolicy
	Nodeselector   map[string]string
	PodSelector    map[string]string
	Affinity       *corev1.Affinity
	Strategy       appv1.DeploymentStrategy
	Tolerations    []corev1.Toleration
	Shareprocess   *bool
	SerivceAccount string
}

type DeployContainerCfg struct {
	Env         []corev1.EnvVar
	Image       string
	Resource    a2t.ExternalResource
	Probe       *corev1.Probe
	ReadyProbe  *corev1.Probe
	Name        string
	Command     []string
	Pullpolicy  corev1.PullPolicy
	Securityctx *corev1.SecurityContext
}

func NewTemplate(alb *a2t.ALB2, cur *appv1.Deployment, albcf *config.ALB2Config, cfg config.OperatorCfg, log logr.Logger) *DeplTemplate {
	return &DeplTemplate{
		alb:        alb,
		name:       alb.Name,
		ns:         alb.Namespace,
		baseDomain: cfg.BaseDomain,
		env:        cfg,
		cur:        cur,
		albcfg:     albcf,
		log:        log,
	}
}

type VolumeCfg struct {
	Volumes map[string]corev1.Volume
	Mounts  map[string]map[string]string
}

func findContainer(name string, depl *appv1.Deployment) *corev1.Container {
	for _, c := range depl.Spec.Template.Spec.Containers {
		if c.Name == name {
			return &c
		}
	}
	return nil
}

func VolumeCfgFromDepl(d *appv1.Deployment) VolumeCfg {
	vcfg := VolumeCfg{
		Volumes: map[string]corev1.Volume{},
		Mounts:  map[string]map[string]string{},
	}
	for _, v := range d.Spec.Template.Spec.Volumes {
		vcfg.Volumes[v.Name] = v
	}
	for _, m := range d.Spec.Template.Spec.Containers {
		if vcfg.Mounts[m.Name] == nil {
			vcfg.Mounts[m.Name] = map[string]string{}
		}
		for _, v := range m.VolumeMounts {
			vcfg.Mounts[m.Name][v.Name] = v.MountPath
		}
	}
	return vcfg
}

func pickConfigFromDeploy(dep *appv1.Deployment) *DeployCfg {
	if dep == nil {
		return nil
	}
	fromContainer := func(c *corev1.Container) DeployContainerCfg {
		if c == nil {
			return DeployContainerCfg{}
		}

		return DeployContainerCfg{
			Name:        c.Name,
			Image:       c.Image,
			Command:     c.Command,
			Pullpolicy:  c.ImagePullPolicy,
			Securityctx: c.SecurityContext,
			Env:         c.Env,
			Probe:       c.LivenessProbe,
			ReadyProbe:  c.ReadinessProbe,
			Resource:    toResource(c.Resources),
		}
	}

	albcfg := fromContainer(findContainer("alb2", dep))
	nginxcfg := fromContainer(findContainer("nginx", dep))
	return &DeployCfg{
		Spec: DeploySpec{
			Name:           dep.Name,
			Ns:             dep.Namespace,
			Owner:          dep.OwnerReferences,
			Replica:        dep.Spec.Replicas,
			Hostnetwork:    dep.Spec.Template.Spec.HostNetwork,
			PodLabel:       dep.Spec.Template.Labels,
			DeployLabel:    dep.Labels,
			Nodeselector:   dep.Spec.Template.Spec.NodeSelector,
			PodSelector:    dep.Spec.Template.Labels,
			Strategy:       dep.Spec.Strategy,
			Tolerations:    dep.Spec.Template.Spec.Tolerations,
			Shareprocess:   dep.Spec.Template.Spec.ShareProcessNamespace,
			Affinity:       dep.Spec.Template.Spec.Affinity,
			SerivceAccount: dep.Spec.Template.Spec.ServiceAccountName,
		},
		Alb:   albcfg,
		Nginx: nginxcfg,
		Vol:   VolumeCfgFromDepl(dep),
	}
}

func (d *DeplTemplate) expectConfig() DeployCfg {
	conf := d.albcfg
	_, alb, nginx := patch.GenImagePatch(conf, d.env)
	pullpolicy := d.env.ImagePullPolicy
	replicas := int32(d.albcfg.Deploy.Replicas)
	hostnetwork := conf.Controller.NetworkMode == a2t.HOST_MODE
	dns := corev1.DNSClusterFirst
	if hostnetwork {
		dns = corev1.DNSClusterFirstWithHostNet
	}
	defaultTolerations := []corev1.Toleration{
		{
			Effect:   corev1.TaintEffect("NoSchedule"),
			Key:      "node-role.kubernetes.io/master",
			Operator: corev1.TolerationOperator("Exists"),
		},
		{
			Effect:   corev1.TaintEffect("NoSchedule"),
			Key:      "node-role.kubernetes.io/control-plane",
			Operator: corev1.TolerationOperator("Exists"),
		},
		{
			Effect:   corev1.TaintEffect("NoSchedule"),
			Key:      "node-role.kubernetes.io/cpaas-system",
			Operator: corev1.TolerationOperator("Exists"),
		},
		{
			Effect:   corev1.TaintEffect("NoSchedule"),
			Key:      "node.kubernetes.io/not-ready",
			Operator: corev1.TolerationOperator("Exists"),
		},
	}

	if _, ok := d.alb.Annotations["tolerate-all"]; ok {
		defaultTolerations = []corev1.Toleration{
			{
				Operator: corev1.TolerationOperator("Exists"),
			},
		}
	}

	return DeployCfg{
		Spec: DeploySpec{
			Name:         d.name,
			Ns:           d.ns,
			Owner:        MakeOwnerRefs(d.alb),
			Replica:      &replicas,
			Hostnetwork:  hostnetwork,
			Dnspolicy:    dns,
			PodLabel:     d.expectPodLabel(),
			DeployLabel:  ALB2ResourceLabel(d.alb.Namespace, d.alb.Name, d.env.Version),
			Nodeselector: conf.Deploy.NodeSelector,
			PodSelector:  d.podSelector(),
			Strategy: appv1.DeploymentStrategy{
				Type: appv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appv1.RollingUpdateDeployment{
					MaxSurge:       &defaultMaxSurge,
					MaxUnavailable: &defaultMaxUnavailable,
				},
			},
			Tolerations:    defaultTolerations,
			Shareprocess:   pointer.To(true),
			Affinity:       d.GenExpectAffinity(),
			SerivceAccount: fmt.Sprintf(FMT_SA, d.alb.Name),
		},
		Alb: DeployContainerCfg{
			Name:  "alb2",
			Image: alb,
			Command: []string{
				"/alb/ctl/run-alb.sh",
			},
			Pullpolicy: corev1.PullPolicy(pullpolicy),
			Securityctx: &corev1.SecurityContext{
				ReadOnlyRootFilesystem: pointer.To(true),
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{
						"SYS_PTRACE", // 向其他process 发信号需要ptrace
						"NET_ADMIN",  // ss 需要 netadmin 和netraw
						"NET_RAW",
						"NET_BIND_SERVICE",
					},
				},
				AllowPrivilegeEscalation: pointer.To(true),
			},
			Env:      conf.GetALBContainerEnvs(),
			Resource: toResource(conf.Deploy.ALbResource),
		},
		Nginx: DeployContainerCfg{
			Name:  "nginx",
			Image: nginx,
			Command: []string{
				"/alb/nginx/run-nginx.sh",
			},
			Pullpolicy: corev1.PullPolicy(pullpolicy),
			Securityctx: &corev1.SecurityContext{
				ReadOnlyRootFilesystem: pointer.To(true),
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{
						"SYS_PTRACE", // 向其他process 发信号需要ptrace
						"NET_ADMIN",  // ss 需要 netadmin 和netraw
						"NET_RAW",
						"NET_BIND_SERVICE",
					},
				},
				AllowPrivilegeEscalation: pointer.To(true),
			},
			Env: conf.GetNginxContainerEnvs(),
			ReadyProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.IntOrString{IntVal: int32(conf.Controller.MetricsPort)},
					},
				},
				InitialDelaySeconds: 3,
				TimeoutSeconds:      5,
				PeriodSeconds:       5,
				SuccessThreshold:    1,
				FailureThreshold:    5,
			},
			Probe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					TCPSocket: &corev1.TCPSocketAction{
						Port: intstr.IntOrString{IntVal: int32(conf.Controller.MetricsPort)},
					},
				},
				InitialDelaySeconds: 60,
				TimeoutSeconds:      5,
				PeriodSeconds:       60,
				SuccessThreshold:    1,
				FailureThreshold:    5,
			},
			Resource: toResource(conf.Deploy.NginxResource),
		},
		Vol: VolumeCfg{
			Volumes: map[string]corev1.Volume{
				"tweak-conf": d.configmapVolume(d.name),
				"share-conf": d.shareVolume(),
				"nginx-run":  d.nginxRunVolume(),
			},
			Mounts: map[string]map[string]string{
				ALB_CONTAINER_NAME: {
					"share-conf": "/etc/alb2/nginx/",
					"tweak-conf": "/alb/tweak/",
				},
				NGINX_CONTAINER_NAME: {
					"share-conf": "/etc/alb2/nginx/",
					"tweak-conf": "/alb/tweak/",
					"nginx-run":  "/alb/nginx/run/",
				},
			},
		},
	}
}

func toResource(res corev1.ResourceRequirements) a2t.ExternalResource {
	return a2t.ExternalResource{
		Limits: &a2t.ContainerResource{
			CPU:    res.Limits.Cpu().String(),
			Memory: res.Limits.Memory().String(),
		},
		Requests: &a2t.ContainerResource{
			CPU:    res.Requests.Cpu().String(),
			Memory: res.Requests.Memory().String(),
		},
	}
}

func fromResource(res a2t.ExternalResource) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(res.Limits.CPU),
			corev1.ResourceMemory: resource.MustParse(res.Limits.Memory),
		},
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(res.Requests.CPU),
			corev1.ResourceMemory: resource.MustParse(res.Requests.Memory),
		},
	}
}

func (d *DeplTemplate) Generate() *appv1.Deployment {
	cfg := d.expectConfig()
	depl := &appv1.Deployment{}
	if !IsNil(d.cur) {
		depl = d.cur
	}
	depl.TypeMeta = v1.TypeMeta{
		Kind:       "Deployment",
		APIVersion: "apps/v1",
	}
	depl.ObjectMeta = v1.ObjectMeta{
		Name:      cfg.Spec.Name,
		Namespace: cfg.Spec.Ns,
	}

	vols := []corev1.Volume{}
	for _, v := range cfg.Vol.Volumes {
		vols = append(vols, v)
	}
	depl.Spec.Template.Spec.Volumes = vols

	depl.Labels = cfg.Spec.DeployLabel

	spec := &depl.Spec.Template.Spec

	depl.Labels = cfg.Spec.DeployLabel
	depl.Annotations = cfg.Spec.DeplAnnotation

	depl.Spec.Replicas = cfg.Spec.Replica

	depl.OwnerReferences = cfg.Spec.Owner

	spec.HostNetwork = cfg.Spec.Hostnetwork
	spec.DNSPolicy = cfg.Spec.Dnspolicy
	if len(cfg.Spec.Nodeselector) == 0 {
		spec.NodeSelector = nil
	} else {
		spec.NodeSelector = cfg.Spec.Nodeselector
	}
	depl.Spec.Selector = &metav1.LabelSelector{MatchLabels: cfg.Spec.PodSelector}
	depl.Spec.Template.Labels = cfg.Spec.PodLabel

	spec.Affinity = cfg.Spec.Affinity
	spec.ServiceAccountName = cfg.Spec.SerivceAccount

	depl.Spec.Strategy = cfg.Spec.Strategy

	spec.Tolerations = cfg.Spec.Tolerations
	spec.ShareProcessNamespace = cfg.Spec.Shareprocess

	alb := cfg.Alb
	nginx := cfg.Nginx
	albMounts := []corev1.VolumeMount{}
	nginxMounts := []corev1.VolumeMount{}
	for name, path := range cfg.Vol.Mounts[ALB_CONTAINER_NAME] {
		albMounts = append(albMounts, corev1.VolumeMount{
			Name:      name,
			MountPath: path,
		})
	}
	for name, path := range cfg.Vol.Mounts[NGINX_CONTAINER_NAME] {
		nginxMounts = append(nginxMounts, corev1.VolumeMount{
			Name:      name,
			MountPath: path,
		})
	}

	spec.Containers = []corev1.Container{
		{
			Name:                     nginx.Name,
			Image:                    nginx.Image,
			Command:                  nginx.Command,
			ImagePullPolicy:          nginx.Pullpolicy,
			SecurityContext:          nginx.Securityctx,
			Resources:                fromResource(nginx.Resource),
			LivenessProbe:            nginx.Probe,
			ReadinessProbe:           nginx.ReadyProbe,
			TerminationMessagePath:   "/dev/termination-log",
			TerminationMessagePolicy: "File",
			VolumeMounts:             nginxMounts,
			Env:                      cfg.Nginx.Env,
		},
		{
			Env:                      cfg.Alb.Env,
			Name:                     alb.Name,
			Image:                    alb.Image,
			Command:                  alb.Command,
			ImagePullPolicy:          alb.Pullpolicy,
			SecurityContext:          alb.Securityctx,
			Resources:                fromResource(alb.Resource),
			LivenessProbe:            alb.Probe,
			ReadinessProbe:           alb.ReadyProbe,
			VolumeMounts:             albMounts,
			TerminationMessagePath:   "/dev/termination-log",
			TerminationMessagePolicy: "File",
		},
	}
	return depl
}

func (b *DeplTemplate) configmapVolume(cmName string) corev1.Volume {
	return corev1.Volume{
		Name: "tweak-conf",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cmName,
				},
				DefaultMode: pointer.To(int32(420)), // 0644
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

func (b *DeplTemplate) shareVolume() corev1.Volume {
	return corev1.Volume{
		Name: "share-conf",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func (b *DeplTemplate) nginxRunVolume() corev1.Volume {
	return corev1.Volume{
		Name: "nginx-run",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func NeedUpdate(cur, latest *appv1.Deployment, log logr.Logger) (bool, string) {
	if cur == nil || latest == nil {
		return false, "is nill"
	}
	curCfg := pickConfigFromDeploy(cur)
	newCfg := pickConfigFromDeploy(latest)
	eq := reflect.DeepEqual(curCfg, newCfg)
	diff := cmp.Diff(curCfg, newCfg)
	log.Info("check deployment change", "diff", diff, "deep-eq", eq)
	if eq {
		return false, ""
	}
	return true, diff
}
