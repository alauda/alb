/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2beta1

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

import (
	"encoding/json"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	ALB2Kind = "ALB2"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:singular=alaudaloadbalancer2,path=alaudaloadbalancer2,shortName=alb2,scope=Namespaced
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:validation:Optional

// ALB2 is the Schema for the alaudaloadbalancer2 API
type ALB2 struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ALB2Spec   `json:"spec,omitempty"`
	Status ALB2Status `json:"status,omitempty"`
}

// ALB2Spec defines the desired state of ALB2
type ALB2Spec struct {
	// custom address of this alb
	Address string `json:"address,omitempty"`
	// +kubebuilder:validation:Enum=nginx
	Type string `json:"type"`
	// +kubebuilder:validation:XPreserveUnknownFields
	Config *ExternalAlbConfig `json:"config"`
}

const (
	HOST_MODE      = "host"
	CONTAINER_MODE = "container"
)

// 这里将所有的fied都设置为指针，这样只是为了方便merge
// crd中所有的字段默认都是optional是通过注解完成的。
// 我们配置了XPreserveUnknownFields 所以这个config可以被任意的增加和删除，主要保证真正使用的时候有默认值即可

type ExternalAlbConfig struct {
	LoadbalancerName     *string            `yaml:"loadbalancerName" json:"loadbalancerName,omitempty"` // # keep compatibility. use meta.name instead
	Vip                  *VipConfig         `yaml:"vip" json:"vip,omitempty"`
	NetworkMode          *string            `yaml:"networkMode" json:"networkMode,omitempty"`
	NodeSelector         map[string]string  `yaml:"nodeSelector" json:"nodeSelector,omitempty"`
	LoadbalancerType     *string            `yaml:"loadbalancerType" json:"loadbalancerType,omitempty"`
	Replicas             *int               `yaml:"replicas" json:"replicas,omitempty"`
	EnableGC             *string            `yaml:"enableGC" json:"enableGC,omitempty"`                       // # 可以删掉 默认关闭
	EnableGCAppRule      *string            `yaml:"enableGCAppRule" json:"enableGCAppRule,omitempty"`         // # 可以删掉 默认关闭
	EnableAlb            *bool              `yaml:"enableALb" json:"enableALb,omitempty"`                     // 是否reconcile ft rule,只有在纯gateway模式时才是false
	EnablePrometheus     *string            `yaml:"enablePrometheus" json:"enablePrometheus,omitempty"`       // openresty的prometheus metrics   TODO 可以删掉 默认为true 应该不会有要关闭metrics的情况
	MetricsPort          *int               `yaml:"metricsPort" json:"metricsPort,omitempty"`                 // openresty的metrics端口 在不同集群可能是不同
	EnablePortprobe      *string            `yaml:"enablePortprobe" json:"enablePortprobe,omitempty"`         // 检查是否有重复的端口  默认开启
	EnableIPV6           *string            `yaml:"enableIPV6" json:"enableIPV6,omitempty"`                   // 可以删掉 默认开启 这个是传递给nginx去绑定ipv6地址的
	EnableHTTP2          *string            `yaml:"enableHTTP2" json:"enableHTTP2,omitempty"`                 // 可以删掉 默认开启
	EnableIngress        *string            `yaml:"enableIngress" json:"enableIngress,omitempty"`             // 是否reconcile ingress
	DefaultIngressClass  *bool              `yaml:"defaultIngressClass" json:"defaultIngressClass,omitempty"` // 是否设置为默认的ingressclass
	EnableCrossClusters  *string            `yaml:"enableCrossClusters" json:"enableCrossClusters,omitempty"` // 在拿ep时是否同时也去拿submariner的svc的ep
	EnableGzip           *string            `yaml:"enableGzip" json:"enableGzip,omitempty"`                   // 可以删掉 默认开启
	DefaultSSLCert       *string            `yaml:"defaultSSLCert" json:"defaultSSLCert,omitempty"`
	DefaultSSLStrategy   *string            `yaml:"defaultSSLStrategy" json:"defaultSSLStrategy,omitempty"`
	IngressHTTPPort      *int               `yaml:"ingressHTTPPort" json:"ingressHTTPPort,omitempty"`
	IngressHTTPSPort     *int               `yaml:"ingressHTTPSPort" json:"ingressHTTPSPort,omitempty"`
	IngressController    *string            `yaml:"ingressController" json:"ingressController,omitempty"` // ingressclass的controller name # TODO 考虑去掉这个配置 ingressclass的controler就是应该是alb的name 不应该能配置
	EnableGoMonitor      *bool              `yaml:"enableGoMonitor" json:"enableGoMonitor,omitempty"`     // 现在的监控实际上是nginx的监控 gomonitor指的是alb自己的监控 现在默认是关闭的，原因见https://jira.alauda.cn/browse/ACP-22070
	EnableProfile        *bool              `yaml:"enableProfile" json:"enableProfile,omitempty"`         // 开启gomonitor的情况下是否开启profile，profile可能暴露敏感信息，所以只能是debug事手动开启
	GoMonitorPort        *int               `yaml:"goMonitorPort" json:"goMonitorPort,omitempty"`
	WorkerLimit          *int               `yaml:"workerLimit" json:"workerLimit,omitempty"`
	ResyncPeriod         *int               `yaml:"resyncPeriod" json:"resyncPeriod,omitempty"`                 // 可以删掉 用默认值就行 还没遇到过需要配置的
	SyncPolicyInterval   *int               `yaml:"syncPolicyInterval" json:"syncPolicyInterval,omitempty"`     // 可以删掉 用默认值就行 还没遇到过需要配置的
	CleanMetricsInterval *int               `yaml:"cleanMetricsInterval" json:"cleanMetricsInterval,omitempty"` // 可以删掉 用默认值就行 还没遇到过需要配置的
	Backlog              *int               `yaml:"backlog" json:"backlog,omitempty"`                           // 可以删掉 用默认值就行 还没遇到过需要配置的
	MaxTermSeconds       *int               `yaml:"maxTermSeconds" json:"maxTermSeconds,omitempty"`             // 可以删掉 用默认值就行 还没遇到过需要配置的
	ReloadTimeout        *int               `yaml:"reloadtimeout" json:"reloadtimeout,omitempty"`               // 每次生成配置的最大超时时间
	PolicyZip            *bool              `yaml:"policyZip" json:"policyZip,omitempty"`                       // zip policy.new 规避安全审查 防止明文policy
	Gateway              *ExternalGateway   `yaml:"gateway" json:"gateway,omitempty"`
	Resources            *ExternalResources `yaml:"resources" json:"resources,omitempty"`
	Projects             []string           `yaml:"projects" json:"projects,omitempty"`
	EnablePortProject    *bool              `yaml:"enablePortProject" json:"enablePortProject,omitempty"` // 是否是端口模式的alb
	PortProjects         *string            `yaml:"portProjects" json:"portProjects,omitempty"`           //   '[{"port":"113-333","projects":["cong"]}]'
	AntiAffinityKey      *string            `yaml:"antiAffinityKey" json:"antiAffinityKey,omitempty"`
	BindNIC              *string            `yaml:"bindNIC" json:"bindNIC,omitempty"` // json string alb绑定网卡的配置 '{"nic":["eth0"]}'
	Overwrite            *ExternalOverwrite `yaml:"overwrite" json:"overwrite,omitempty"`
}

type VipConfig struct {
	EnableLbSvc                   bool                   `yaml:"enableLbSvc" json:"enableLbSvc,omitempty"`
	LbSvcIpFamilyPolicy           *corev1.IPFamilyPolicy `yaml:"lbSvcIpFamilyPolicy" json:"lbSvcIpFamilyPolicy,omitempty"`
	AllocateLoadBalancerNodePorts *bool                  `yaml:"allocateLoadBalancerNodePorts" json:"allocateLoadBalancerNodePorts,omitempty"`
	LbSvcAnnotations              map[string]string      `yaml:"lbSvcAnnotations" json:"lbSvcAnnotations,omitempty"`
}

type GatewayMode string

const (
	GatewayModeShared     GatewayMode = "shared"
	GatewayModeStandAlone GatewayMode = "standalone"
)

type ExternalGateway struct {
	Enable *bool        `yaml:"enable" json:"enable,omitempty"`
	Mode   *GatewayMode `yaml:"mode" json:"mode,omitempty"`
	Name   *string      `yaml:"name" json:"name,omitempty"`
}

type ContainerResource struct {
	CPU    string `yaml:"cpu" json:"cpu,omitempty"`
	Memory string `yaml:"memory" json:"memory,omitempty"`
}

func (c *ContainerResource) UnmarshalJSON(data []byte) error {
	type FixedResources struct {
		CPU    intstr.IntOrString `yaml:"cpu" json:"cpu,omitempty"`
		Memory intstr.IntOrString `yaml:"memory" json:"memory,omitempty"`
	}
	res := FixedResources{}
	if err := json.Unmarshal(data, &res); err != nil {
		return err
	}
	if res.CPU.String() != "0" {
		c.CPU = res.CPU.String()
	}
	if res.Memory.String() != "0" {
		c.Memory = res.Memory.String()
	}
	return nil
}

type ExternalResource struct {
	Limits   *ContainerResource `yaml:"limits" json:"limits,omitempty"`
	Requests *ContainerResource `yaml:"requests" json:"requests,omitempty"`
}

type ExternalResources struct {
	Alb               *ExternalResource `yaml:"alb" json:"alb,omitempty"`
	*ExternalResource `json:",inline,omitempty"`
}

type ExternalOverwrite struct {
	Image     []ExternalImageOverwriteConfig     `yaml:"image" json:"image"`
	Configmap []ExternalConfigmapOverwriteConfig `yaml:"configmap" json:"configmap"`
}

type ExternalImageOverwriteConfig struct {
	Target string `yaml:"target,omitempty" json:"target"` // 指定要覆盖的某个具体版本的alb的image,如果没有写的话，则在每个版本上都覆盖
	Alb    string `yaml:"alb" json:"alb"`
	Nginx  string `yaml:"nginx" json:"nginx"`
}

type ExternalConfigmapOverwriteConfig struct {
	Target string `yaml:"target,omitempty" json:"target"` // 指定要覆盖的某个具体版本的alb的configmap,如果没有写的话，则在每个版本上都覆盖
	Name   string `yaml:"name" json:"name"`               // ns/name key of configmap
}

type ALB2State string

const (
	ALB2StateRunning     = ALB2State("Running")
	ALB2StateProgressing = ALB2State("Progressing")
	ALB2StatePending     = ALB2State("Pending")
	ALB2StateWarning     = ALB2State("Warning")
)

// ALB2Status defines the observed state of ALB2, detail in ALB2StatusDetail
type ALB2Status struct {
	// state defines the status of alb2, the possible values are ready/warning
	// state:ready means ok
	// state:warning can be caused by port conflict in alb2
	// +kubebuilder:default="Pending"
	State ALB2State `json:"state"`
	// reason defines the possible cause of alb2 state change
	Reason    string `json:"reason"`
	ProbeTime int64  `json:"probeTime"`
	// +optional
	ProbeTimeStr metav1.Time `json:"probeTimeStr"`
	// +optional
	Detail ALB2StatusDetail `json:"detail,omitempty"`
}

type ALB2StatusDetail struct {
	// status set by operator
	// +optional
	Deploy DeployStatus `json:"deploy"`
	// status set by alb itself
	// +optional
	Alb AlbStatus `json:"alb"`
	// status set by operator
	// +optional
	AddressStatus AssignedAddress `json:"address"`
	// status set by operator
	// +optional
	Versions VersionStatus `json:"version"`
}

type VersionStatus struct {
	Version    string `json:"version"`    // 这个alb的operator的chart的版本号
	ImagePatch string `json:"imagePatch"` // 是否设置了image patch
}

type DeployStatus struct {
	State        ALB2State   `json:"state"`
	Reason       string      `json:"reason"`
	ProbeTimeStr metav1.Time `json:"probeTimeStr"`
}

type AlbStatus struct {
	// port status of this alb. key format protocol-port
	// +optional
	PortStatus map[string]PortStatus `json:"portstatus"`
}

type AssignedAddress struct {
	Ok   bool     `json:"ok"`
	Msg  string   `json:"msg"`
	Ipv4 []string `json:"ipv4"`
	Ipv6 []string `json:"ipv6"`
	Host []string `json:"host"`
}

type PortStatus struct {
	Conflict     bool        `json:"conflict"`
	Msg          string      `json:"msg"`
	ProbeTimeStr metav1.Time `json:"probeTimeStr"`
}

//+kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ALB2List contains a list of ALB2
type ALB2List struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ALB2 `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ALB2{}, &ALB2List{})
}

func (alb *ALB2) GetAllAddress() []string {
	address := strings.Split(alb.Spec.Address, ",")
	address = append(address, alb.Status.Detail.AddressStatus.Ipv4...)
	address = append(address, alb.Status.Detail.AddressStatus.Ipv6...)
	address = append(address, alb.Status.Detail.AddressStatus.Host...)

	ret := []string{}
	for _, addr := range address {
		if strings.TrimSpace(addr) == "" {
			continue
		}
		ret = append(ret, addr)
	}
	return ret
}
