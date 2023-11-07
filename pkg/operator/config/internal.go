package config

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-logr/logr"

	. "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	. "alauda.io/alb2/pkg/config"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type OverwriteCfg ExternalOverwrite

// 现在修改一个配置修改更新3个地方
//  1. alb v2bta1的cr
//  2. external的默认配置
//  3. albrun config用的fromenv/toenv

// tree like config use in operator.
type ALB2Config struct {
	ALBRunConfig
	Project   ProjectConfig // project现在在alb内是直接从label中拿的,端口项目是直接从ft的label上拿的,albrun不关心projectcfg
	Vip       VipConfig     // albrun 也不关心lbsvc的annotation
	Deploy    DeployConfig
	Overwrite OverwriteCfg
	Flags     OperatorFlags
	ExtraConfig
}

type OperatorFlags struct {
	DefaultIngressClass bool
}

type ExtraConfig struct {
	BindNic string //bindnic 是写在configmap然后volume的,不是环境变量
}

type ProjectConfig struct {
	EnablePortProject bool     `json:"enablePortProject,omitempty"`
	PortProjects      string   `json:"portProjects,omitempty"` // json string of []struct {Port:stringProjects:[]string}
	Projects          []string `json:"projects,omitempty"`
}

type DeployConfig struct {
	Replicas        int
	AntiAffinityKey string
	ALbResource     coreV1.ResourceRequirements
	NginxResource   coreV1.ResourceRequirements
	NodeSelector    map[string]string
}

func NewALB2Config(albCr *ALB2, op OperatorCfg, log logr.Logger) (*ALB2Config, error) {
	external, err := NewExternalAlbConfigWithDefault(albCr)
	if err != nil {
		return nil, err
	}
	cfg := &ALB2Config{
		ALBRunConfig: ALBRunConfig{
			Name:   albCr.Name,
			Ns:     albCr.Namespace,
			Domain: op.BaseDomain,
		},
	}
	err = cfg.Merge(external)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (a *ALB2Config) Merge(ec ExternalAlbConfig) error {
	a.Name = *ec.LoadbalancerName
	a.Deploy = DeployConfig{}
	a.Project = ProjectConfig{}
	a.Gateway = GatewayConfig{}
	a.Vip = VipConfig{}
	// NOTE: the merge order are matters,som merge will change other's cfg
	if ec.Vip != nil {
		a.Vip = *ec.Vip
	}
	MergeController(ec, a)
	err := MergeDeploy(ec, a)
	if err != nil {
		return err
	}
	mergeExtra(ec, a)
	MergeProject(ec, a)
	gatewayFromCr(ec, a)
	if ec.Overwrite != nil {
		a.Overwrite = OverwriteCfg(*ec.Overwrite)
	}
	a.Flags = mergeOperatorFlags(ec, a)
	return nil
}

func mergeExtra(ec ExternalAlbConfig, a *ALB2Config) {
	a.BindNic = *ec.BindNIC //bindnic 是从volume中读的
}

func mergeOperatorFlags(ec ExternalAlbConfig, a *ALB2Config) OperatorFlags {
	defaultClass := false
	if ec.DefaultIngressClass != nil {
		defaultClass = *ec.DefaultIngressClass
	}
	return OperatorFlags{
		DefaultIngressClass: defaultClass,
	}
}

func MergeController(ec ExternalAlbConfig, a *ALB2Config) {
	c := &a.Controller
	c.BackLog = *ec.Backlog
	c.NetworkMode = *ec.NetworkMode
	c.MetricsPort = *ec.MetricsPort
	c.HttpPort = *ec.IngressHTTPPort
	c.HttpsPort = *ec.IngressHTTPSPort
	c.SSLCert = *ec.DefaultSSLCert
	c.DefaultSSLStrategy = *ec.DefaultSSLStrategy
	c.MaxTermSeconds = *ec.MaxTermSeconds
	c.ReloadTimeout = *ec.ReloadTimeout
	c.CpuPreset = CpuPresetToCore(ec.Resources.Limits.CPU)
	c.WorkerLimit = *ec.WorkerLimit
	c.ResyncPeriod = *ec.ResyncPeriod
	c.GoMonitorPort = *ec.GoMonitorPort
	c.Flags = ControllerFlags{}
	flagsFromCr(ec, a)
}

func MergeDeploy(ec ExternalAlbConfig, a *ALB2Config) error {
	d := &a.Deploy
	ngxLimitCpu, err := resource.ParseQuantity(ec.Resources.Limits.CPU)
	if err != nil {
		return err
	}
	ngxLimMem, err := resource.ParseQuantity(ec.Resources.Limits.Memory)
	if err != nil {
		return err
	}
	ngxReqCpu, err := resource.ParseQuantity(ec.Resources.Requests.CPU)
	if err != nil {
		return err
	}
	ngxReqMem, err := resource.ParseQuantity(ec.Resources.Requests.Memory)
	if err != nil {
		return err
	}

	albLimitCpu, err := resource.ParseQuantity(ec.Resources.Alb.Limits.CPU)
	if err != nil {
		return err
	}
	albLimitMem, err := resource.ParseQuantity(ec.Resources.Alb.Limits.Memory)
	if err != nil {
		return err
	}
	albReqCpu, err := resource.ParseQuantity(ec.Resources.Alb.Requests.CPU)
	if err != nil {
		return err
	}
	albReqMem, err := resource.ParseQuantity(ec.Resources.Alb.Requests.Memory)
	if err != nil {
		return err
	}
	d.NginxResource = coreV1.ResourceRequirements{
		Limits: coreV1.ResourceList{
			coreV1.ResourceCPU:    ngxLimitCpu,
			coreV1.ResourceMemory: ngxLimMem,
		},
		Requests: coreV1.ResourceList{
			coreV1.ResourceCPU:    ngxReqCpu,
			coreV1.ResourceMemory: ngxReqMem,
		},
	}
	d.ALbResource = coreV1.ResourceRequirements{
		Limits: coreV1.ResourceList{
			coreV1.ResourceCPU:    albLimitCpu,
			coreV1.ResourceMemory: albLimitMem,
		},
		Requests: coreV1.ResourceList{
			coreV1.ResourceCPU:    albReqCpu,
			coreV1.ResourceMemory: albReqMem,
		},
	}
	d.Replicas = *ec.Replicas
	d.AntiAffinityKey = *ec.AntiAffinityKey
	d.NodeSelector = ec.NodeSelector
	return nil
}

func flagsFromCr(ec ExternalAlbConfig, a *ALB2Config) {
	f := &a.Controller.Flags
	f.EnableAlb = toBool(*ec.EnableAlb)
	f.EnableGC = toBool(*ec.EnableGC)
	f.EnableGCAppRule = toBool(*ec.EnableGCAppRule)
	f.EnablePrometheus = toBool(*ec.EnablePrometheus)
	f.EnablePortProbe = toBool(*ec.EnablePortprobe)
	f.EnablePortProject = toBool(*ec.EnablePortProject)
	f.EnableIPV6 = toBool(*ec.EnableIPV6)
	f.EnableHTTP2 = toBool(*ec.EnableHTTP2)
	f.EnableCrossClusters = toBool(*ec.EnableCrossClusters)
	f.EnableGzip = toBool(*ec.EnableGzip)
	f.EnableGoMonitor = toBool(*ec.EnableGoMonitor)
	f.EnableProfile = toBool(*ec.EnableProfile)
	f.EnableIngress = toBool(*ec.EnableIngress)
	f.PolicyZip = toBool(*ec.PolicyZip)
	f.EnableLbSvc = a.Vip.EnableLbSvc
}

func MergeProject(ec ExternalAlbConfig, a *ALB2Config) {
	d := &a.Project
	d.EnablePortProject = *ec.EnablePortProject
	d.Projects = ec.Projects
	d.PortProjects = *ec.PortProjects
}

func gatewayFromCr(ec ExternalAlbConfig, a *ALB2Config) {
	g := &a.Gateway
	if ec.Gateway == nil {
		return
	}
	// 如果是显示的关闭
	if ec.Gateway.Enable != nil && !*ec.Gateway.Enable {
		return
	}
	// 如果既没有enable也没有mode配置
	if ec.Gateway.Enable == nil && ec.Gateway.Mode == nil {
		return
	}

	// 兼容性配置
	// 当gateway模式开启，但是没有指定gatewaymode的时候，默认用共享模式
	// 共享模式下name默认是alb的name
	// standalone模式下要有指定的name
	// defaultGatewayClass =
	if ec.Gateway.Mode == nil || *ec.Gateway.Mode == GatewayModeShared {
		g.Enable = true
		g.Mode = GatewayModeShared
		g.Shared = &GatewaySharedConfig{}
		if ec.Gateway.Name != nil {
			g.Shared.GatewayClassName = *ec.Gateway.Name
		} else {
			g.Shared.GatewayClassName = *ec.LoadbalancerName
		}
		return
	}

	if ec.Gateway.Mode != nil && *ec.Gateway.Mode == GatewayModeStandAlone && ec.Gateway.Name != nil {
		g.Enable = true
		g.Mode = GatewayModeStandAlone
		names := strings.Split(*ec.Gateway.Name, "/")
		name := a.Name
		ns := a.Ns
		if len(names) == 1 {
			name = names[0]
		}
		if len(names) == 2 {
			name = names[0]
			ns = names[1]
		}

		g.StandAlone = &GatewayStandAloneConfig{
			GatewayName: name,
			GatewayNS:   ns,
		}
		// 独享型gateway的默认配置
		a.enableVip()
		a.enableContainerMode()
		a.Controller.Flags.EnableAlb = false
		a.Controller.Flags.EnableIngress = false
		a.Controller.Flags.EnablePortProbe = false
	}
}

func (a *ALB2Config) enableVip() {
	// NOTE: 这里改两个flag的原因在于我们希望AlbRunConfig在alb的容器内也是通用的,但是vip配置中的lbsvcannotation是不会传递给alb容器的
	a.Vip.EnableLbSvc = true
	a.Controller.Flags.EnableLbSvc = true
}

func (a *ALB2Config) enableContainerMode() {
	a.Controller.NetworkMode = CONTAINER_MODE
	// 容器网络模式就不用做端口冲突的检测了
	a.Controller.Flags.EnablePortProbe = false
}

func (a *ALB2Config) Show() string {
	return fmt.Sprintf("%+v", a)
}

func toBool(x interface{}) bool {
	switch x.(type) {
	case string:
		bs := strings.ToLower(x.(string))
		return bs == "true"
	case bool:
		return x.(bool)
	default:
		panic("? what is you type")
	}
}

func IsPointToTrue(p *bool) bool {
	if p == nil {
		return false
	}
	return *p
}

func IsPointToFalse(p *bool) bool {
	if p == nil {
		return false
	}
	return !*p
}

func IsPointToString(p *string, expect string) bool {
	if p == nil {
		return false
	}
	return *p == expect
}

// less 1c will be 1. 600m=>1 1800m>2 2000m>2
func CpuPresetToCore(v string) int {
	// cpu limit could have value like 200m, need some calculation
	re := regexp.MustCompile(`([0-9]+)m`)
	var val int
	if string_decimal := strings.TrimRight(re.FindString(fmt.Sprintf("%v", v)), "m"); string_decimal == "" {
		val, _ = strconv.Atoi(v)
	} else {
		val_decimal, _ := strconv.Atoi(string_decimal)
		val = int(math.Ceil(float64(val_decimal) / 1000))
	}
	return val
}
