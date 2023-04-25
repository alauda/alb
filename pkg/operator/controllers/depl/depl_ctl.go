package depl

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/util/intstr"

	albv2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	cfg "alauda.io/alb2/pkg/operator/config"
	patch "alauda.io/alb2/pkg/operator/controllers/depl/patch"
	"alauda.io/alb2/pkg/operator/controllers/depl/resources"
	"alauda.io/alb2/pkg/operator/controllers/depl/resources/configmap"
	rg "alauda.io/alb2/pkg/operator/controllers/depl/resources/gateway"
	"alauda.io/alb2/pkg/operator/controllers/depl/resources/ingress"
	"alauda.io/alb2/pkg/operator/controllers/depl/resources/portinfo"
	"alauda.io/alb2/pkg/operator/controllers/depl/resources/service"
	"alauda.io/alb2/pkg/operator/controllers/depl/resources/workload"
	"alauda.io/alb2/pkg/operator/resourceclient"
	. "alauda.io/alb2/pkg/operator/toolkit"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctlClient "sigs.k8s.io/controller-runtime/pkg/client"
	gateway "sigs.k8s.io/gateway-api/apis/v1alpha2"

	perr "github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type AlbDeployCtl struct {
	Cli *resourceclient.ALB2ResourceClient
	Env cfg.OperatorCfg
	Log logr.Logger
	Cfg *cfg.ALB2Config
}

func NewAlbDeployCtl(Cli ctlClient.Client, env cfg.OperatorCfg, log logr.Logger, cfg *cfg.ALB2Config) AlbDeployCtl {
	return AlbDeployCtl{
		Cli: resourceclient.NewALB2ResourceClient(Cli),
		Env: env,
		Cfg: cfg,
		Log: log,
	}
}

func (d *AlbDeployCtl) GenExpectAlbDeploy(ctx context.Context, cur *AlbDeploy) (*AlbDeploy, error) {
	var err error
	cfg := d.Cfg
	alb, err := d.genExpectAlb(cur, cfg)
	if err != nil {
		return nil, err
	}
	comm, err := d.genExpectConfigmap(ctx, cur, cfg)
	if err != nil {
		return nil, err
	}

	port, err := d.genExpectPortInfoConfigmap(cur, cfg)
	if err != nil {
		return nil, err
	}

	svc, err := d.genExpectSvc(cur, cfg)
	if err != nil {
		return nil, err
	}

	ic, err := d.genExpectIngressClass(cur, cfg)
	if err != nil {
		return nil, err
	}

	gc, err := d.genExpectGatewayClass(cur, cfg)
	if err != nil {
		return nil, err
	}

	depl, err := d.genExpectDeployment(cur, cfg)
	if err != nil {
		return nil, err
	}
	feature := d.genExpectFeature(cur, cfg)
	expect := AlbDeploy{
		Alb:      alb,
		Deploy:   depl,
		Common:   comm,
		PortInfo: port,
		Ingress:  ic,
		Gateway:  gc,
		Svc:      svc,
		Feature:  feature,
	}
	return &expect, nil
}

func (d *AlbDeployCtl) genExpectAlb(cur *AlbDeploy, conf *cfg.ALB2Config) (*albv2.ALB2, error) {
	if cur.Alb == nil {
		return nil, fmt.Errorf("could not find alb ")
	}
	var (
		env             = d.Env
		labelBaseDomain = env.GetLabelBaseDomain()
	)
	alb := cur.Alb.DeepCopy()
	var projectLabel = map[string]string{}

	projectPrefix := fmt.Sprintf("project.%s", labelBaseDomain)
	for _, project := range conf.Project.Projects {
		key := fmt.Sprintf("%s/%s", projectPrefix, project)
		projectLabel[key] = "true"
	}

	if conf.Project.EnablePortProject {
		key := fmt.Sprintf("%s/%s", labelBaseDomain, "role")
		projectLabel[key] = "port"
	}
	alb.Labels = resources.MergeLabel(projectLabel, resources.RemovePrefixKey(alb.Labels, projectPrefix))
	return alb, nil
}

func (d *AlbDeployCtl) genExpectDeployment(cur *AlbDeploy, conf *cfg.ALB2Config) (*appsv1.Deployment, error) {
	var (
		ownerRefs       = MakeOwnerRefs(cur.Alb)
		labelBaseDomain = d.Env.GetLabelBaseDomain()
		refLabel        = resources.ALB2ResourceLabel(cur.Alb.Namespace, cur.Alb.Name, d.Env.Version)
	)

	albEnvs := conf.GetALBContainerEnvs(d.Env)
	nginxEnvs := conf.GetNginxContainerEnvs()

	var generateOptions = []workload.Option{
		workload.SetOwnerRefs(ownerRefs),
		workload.SetEnvs(albEnvs, "alb2"),
		workload.SetEnvs(nginxEnvs, "nginx"),
		workload.SetReplicas(int32(conf.Deploy.Replicas)),
		workload.WithHostNetwork(conf.Controller.NetworkMode == "host"),
		workload.SetNodeSelector(conf.Deploy.NodeSelector),
		workload.SetLabel(refLabel),
		workload.SetLivenessProbe("nginx", &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/status",
					Port:   intstr.IntOrString{IntVal: int32(conf.Controller.MetricsPort)},
					Scheme: "HTTP",
				},
			},
			InitialDelaySeconds: 60,
			TimeoutSeconds:      5,
			PeriodSeconds:       60,
			SuccessThreshold:    1,
			FailureThreshold:    5,
		}),
	}

	hasPatch, alb, nginx := patch.GenImagePatch(conf, d.Env)
	d.Log.Info("image patch", "has", hasPatch, "alb", alb, "nginx", nginx)
	if hasPatch {
		generateOptions = append(generateOptions, workload.SetImage("alb2", alb))
		generateOptions = append(generateOptions, workload.SetImage("nginx", nginx))
	}

	if conf.Deploy.AntiAffinityKey != "" {
		generateOptions = append(generateOptions,
			workload.SetAffinity(conf.Deploy.AntiAffinityKey, conf.Controller.NetworkMode, labelBaseDomain))
	}

	if conf.Deploy.NginxResource != nil {
		generateOptions = append(generateOptions,
			workload.SetResource(*conf.Deploy.NginxResource, "nginx"))
	}
	if conf.Deploy.ALbResource != nil {
		generateOptions = append(generateOptions,
			workload.SetResource(*conf.Deploy.ALbResource, "alb2"))
	}
	deploy := workload.NewTemplate(cur.Alb.Namespace, cur.Alb.Name, labelBaseDomain, cur.Deploy.DeepCopy(), conf, d.Env).Generate(generateOptions...)
	return deploy, nil
}

func (d *AlbDeployCtl) genExpectIngressClass(cur *AlbDeploy, conf *cfg.ALB2Config) (*netv1.IngressClass, error) {
	// TODO ingress class 的controller是不可变的
	var (
		alb2            = cur.Alb
		refLabel        = resources.ALB2ResourceLabel(alb2.Namespace, alb2.Name, d.Env.Version)
		labelBaseDomain = d.Env.GetLabelBaseDomain()
	)
	if !conf.Controller.Flags.EnableIngress {
		return nil, nil
	}
	ic := ingress.NewTemplate(alb2.GetNamespace(), alb2.Name, labelBaseDomain).Generate(
		ingress.AddLabel(refLabel),
	)
	return ic, nil
}

func (d *AlbDeployCtl) genExpectGatewayClass(cur *AlbDeploy, conf *cfg.ALB2Config) (*gateway.GatewayClass, error) {
	var (
		alb2            = cur.Alb
		refLabel        = resources.ALB2ResourceLabel(alb2.Namespace, alb2.Name, d.Env.Version)
		labelBaseDomain = d.Env.GetLabelBaseDomain()
	)
	if !conf.Gateway.Enable {
		return nil, nil
	}
	if conf.Gateway.Mode != "gatewayclass" {
		return nil, nil
	}
	// TODO 像这种互相有依赖的资源，其所需要的是cur的cr还是expect的cr呢？ gatewayclass这里其实就是需要gvk所有用那个都行
	gc := rg.NewTemplate(alb2.Namespace, alb2.Name, labelBaseDomain, alb2, cur.Gateway, d.Log).Generate(
		rg.AddLabel(refLabel),
	)
	return gc, nil
}

func (d *AlbDeployCtl) genExpectConfigmap(ctx context.Context, cur *AlbDeploy, conf *cfg.ALB2Config) (*corev1.ConfigMap, error) {
	// TODO 我们需要考虑configmap break change 的情况,现在是直接更新configmap的，这样有可能旧版本的alb看到了新版本的configmap。。
	//不应该和alb运行相关的配置放在configmap中，比如lua dict之类的
	var (
		ownerRefs = MakeOwnerRefs(cur.Alb)
		bindNic   = conf.Controller.BindNic
		refLabel  = resources.ALB2ResourceLabel(cur.Alb.Namespace, cur.Alb.Name, d.Env.Version)
		alb2      = cur.Alb
	)

	defaultPatches := []configmap.Option{
		configmap.SetOwnerRefs(ownerRefs),
		configmap.WithBindNIC(bindNic),
		configmap.AddLabel(refLabel),
	}
	// 当有patch的时候，我们要用patch
	hasPatch, overwriteConfigmap, err := patch.FindConfigmapPatch(ctx, d.Cli, conf, d.Env)
	if err != nil {
		return nil, err
	}
	albConfigmap := configmap.NewTemplate(alb2.Namespace, alb2.Name).Generate(defaultPatches...)
	if hasPatch {
		// 绑定网卡的配置是从alb的config上获取的，不应该从patch上获取,所以这里不更新bindnic
		keys := []string{"http", "http_server", "grpc_server", "stream-common", "stream-tcp", "stream-udp", "upstream"}
		for _, key := range keys {
			albConfigmap.Data[key] = overwriteConfigmap.Data[key]
		}
	}
	return albConfigmap, nil
}

func (d *AlbDeployCtl) genExpectPortInfoConfigmap(cur *AlbDeploy, conf *cfg.ALB2Config) (*corev1.ConfigMap, error) {
	var (
		alb2      = cur.Alb
		refLabel  = resources.ALB2ResourceLabel(alb2.Namespace, alb2.Name, d.Env.Version)
		ownerRefs = MakeOwnerRefs(alb2)
	)
	data := conf.Project.PortProjects
	portMap := portinfo.NewTemplate(alb2.Namespace, alb2.Name, string(data)).Generate(
		portinfo.AddLabel(refLabel),
		portinfo.SetOwnerRefs(ownerRefs),
	)
	return portMap, nil
}

func GenExpectDeployStatus(deploy *appsv1.Deployment) albv2.DeployStatus {
	deployStatus := albv2.DeployStatus{}
	if deploy == nil {
		deployStatus = albv2.DeployStatus{
			State:        albv2.ALB2StatePending,
			Reason:       "wait workload creating",
			ProbeTimeStr: metav1.Time{Time: time.Now()},
		}
	} else {
		if deploy.Status.Replicas != deploy.Status.ReadyReplicas {
			deployStatus = albv2.DeployStatus{
				State:        albv2.ALB2StateProgressing,
				Reason:       fmt.Sprintf("wait workload ready %v/%v", deploy.Status.ReadyReplicas, deploy.Status.Replicas),
				ProbeTimeStr: metav1.Time{Time: time.Now()},
			}
		}
		if deploy.Status.Replicas == deploy.Status.ReadyReplicas {
			deployStatus = albv2.DeployStatus{
				State:        albv2.ALB2StateRunning,
				Reason:       "workload ready",
				ProbeTimeStr: metav1.Time{Time: time.Now()},
			}
		}
	}
	return deployStatus
}

// 在生成期望的状态时，我们要从旧的状态中拿到alb自身更新的port的状态.
func GenExpectStatus(conf *cfg.ALB2Config, operator cfg.OperatorCfg, originStatus albv2.ALB2Status, deploy *appsv1.Deployment) albv2.ALB2Status {
	versionStatus := albv2.VersionStatus{}
	hasPatch, alb, nginx := patch.GenImagePatch(conf, operator)
	versionStatus.Version = operator.Version
	if hasPatch {
		patchStatus := fmt.Sprintf("patched,alb: %v,nginx: %v", alb, nginx)
		versionStatus.ImagePatch = patchStatus
	} else {
		versionStatus.ImagePatch = "not patch"
	}

	status := originStatus.DeepCopy()
	// TODO 为了显示deploy的状态，我们最好是在deployment发生变化时 reconcile一次
	// 但现在的实现无法精确的判断出是否要重新更新deploymnt，只会变成 alb->deployment->alb的无限循环
	// 正确的做法应该是在创建deployment时，使用一个deploymentcfg，reconcile时判断deploymentcfg是否发生变化
	// 所以现在先不显示deployment的状态
	status.Detail.Deploy = GenExpectDeployStatus(deploy)
	status.Detail.Versions = versionStatus
	MergeAlbStatus(status)
	return *status
}

func MergeAlbStatus(status *albv2.ALB2Status) {
	// 在没有端口冲突的情况下，alb的status就是deployment运行的状态
	// 在有端口冲突的情况下 alb的status是warnning msg是端口冲突信息
	depl := status.Detail.Deploy
	status.State = depl.State
	status.Reason = depl.Reason
	status.Reason = depl.Reason
	status.ProbeTimeStr = metav1.Time{Time: time.Now()}
	status.ProbeTime = time.Now().Unix()
	// port conflict
	albStatus := status.Detail.Alb
	if len(albStatus.PortStatus) != 0 {
		status.State = albv2.ALB2StateWarning
		reason := ""
		for port, msg := range albStatus.PortStatus {
			reason += fmt.Sprintf("%s %s.", port, msg.Msg)
		}
		status.Reason = reason
	}
}

func (d *AlbDeployCtl) genExpectSvc(cur *AlbDeploy, conf *cfg.ALB2Config) (*AlbDeploySvc, error) {
	var (
		alb2      = cur.Alb
		ownerRefs = MakeOwnerRefs(alb2)
		refLabel  = resources.ALB2ResourceLabel(alb2.Namespace, alb2.Name, d.Env.Version)
	)
	// TODO 现在loadbalancer类型的svc可以同时配置tcp和udp了。后期该如何升级呢？
	// TODO refacotr
	var svc *corev1.Service
	if conf.Controller.NetworkMode == "host" {
		labels := refLabel
		// 监控需要svc上有这个label
		labels["service_name"] = fmt.Sprintf("alb2-%s", alb2.Name)
		svc = service.NewTemplate(alb2.Namespace, alb2.Name, "TCP", int32(conf.Controller.MetricsPort)).
			Generate(
				service.SetOwnerRefs(ownerRefs),
				service.SetServiceType(corev1.ServiceTypeClusterIP),
				service.AddLabel(labels),
				func(new *corev1.Service) {
					// 主机网络的时候 svc不应该被其他人更新
					if cur.Svc != nil && cur.Svc.Svc != nil {
						new.ResourceVersion = cur.Svc.Svc.ResourceVersion
					}
				},
			)
	}
	var tcpService *corev1.Service
	var udpService *corev1.Service

	if conf.Controller.NetworkMode == "container" {
		tcpServiceName := FmtKeyBySep("-", alb2.Name, "tcp")
		tcpService = service.NewTemplate(alb2.Namespace, tcpServiceName, "TCP", int32(conf.Controller.MetricsPort)).
			Generate(
				service.SetOwnerRefs(ownerRefs),
				service.SetServiceType(corev1.ServiceTypeLoadBalancer),
				service.AddMetalLBAnnotation(alb2.Namespace, alb2.Name),
				service.AddLabel(refLabel),
			)

		udpServiceName := FmtKeyBySep("-", alb2.Name, "udp")
		udpService = service.NewTemplate(alb2.Namespace, udpServiceName, "UDP", int32(conf.Controller.MetricsPort)).
			Generate(
				service.SetOwnerRefs(ownerRefs),
				service.SetServiceType(corev1.ServiceTypeLoadBalancer),
				service.AddMetalLBAnnotation(alb2.Namespace, alb2.Name),
				service.AddLabel(refLabel),
			)
	}

	return &AlbDeploySvc{
		Svc:    svc,
		TcpSvc: tcpService,
		UdpSvc: udpService,
	}, nil
}

func (d *AlbDeployCtl) genExpectFeature(cur *AlbDeploy, conf *cfg.ALB2Config) *unstructured.Unstructured {
	if !conf.Controller.Flags.EnableIngress {
		return nil
	}
	return FeatureCr(cur.Feature, conf.Name, conf.Ns, conf.Controller.Address)
}

// 对于升级上来的资源，我们可能没有更新他，即没有设置ownerRefs，所以这里还是自己手动删一下
func Destory(ctx context.Context, cli client.Client, log logr.Logger, cur *AlbDeploy) error {
	l := log
	objs := []ctlClient.Object{}
	objs = append(objs, cur.Alb)
	objs = append(objs, cur.Common)
	objs = append(objs, cur.PortInfo)
	objs = append(objs, cur.Deploy)
	objs = append(objs, cur.Gateway)
	objs = append(objs, cur.Ingress)
	if cur.Svc != nil {
		objs = append(objs, cur.Svc.Svc)
		objs = append(objs, cur.Svc.TcpSvc)
		objs = append(objs, cur.Svc.UdpSvc)
	}
	l.Info("delete obj", "alb", cur.Alb.Name, "len", len(objs))
	for _, obj := range objs {
		if IsNil(obj) {
			continue
		}
		l.Info("delete obj", "alb", cur.Alb.Name, "obj", ShowMeta(obj))
		err := cli.Delete(ctx, obj)
		if !errors.IsNotFound(err) {
			l.Error(err, "delete obj fail", "alb", cur.Alb.Name, "obj", ShowMeta(obj))
		}
	}
	return nil
}

// 在将这个alb所以已有的cr和期望的cr都拿到后，我们就可以进行升级，如果需要reconcile return true，nil
// 现在我们都是更新的，所以一次操作就可以升级完成，后续需要操作deployment做scale，可能要reconcile
func (d *AlbDeployCtl) DoUpdate(ctx context.Context, cur *AlbDeploy, expect *AlbDeploy) (reconcile bool, err error) {
	// TODO 这个函数中需要特殊处理的和基本可以忽略的混在一起了 有点乱
	l := d.Log

	// deployment的更新会导致alb的status的更新导致alb的resourceversion的变化,导致这里的当前的alb不是最新的了，所以这里要先更新alb
	// 如果alb需要更新 可能是label或者annotation的变化 我们不会去该spec的内容，那个应该是只由用户操作的
	sameAlb, _ := d.SameAlb(cur.Alb, expect.Alb)
	if !sameAlb {
		l.Info("alb change", "diff", cmp.Diff(cur.Alb, expect.Alb))
		err := d.Cli.Update(ctx, expect.Alb)
		if err != nil {
			return false, err
		}
		l.Info("alb update success", "ver", expect.Alb.ResourceVersion)
		return true, nil
	}
	err = deleteOrCreateOrUpdate(ctx, d.Cli, d.Log, cur.Deploy, expect.Deploy, func(cur *appsv1.Deployment, expect *appsv1.Deployment) bool {
		if cur.Namespace != expect.Namespace {
			return true
		}
		if cur.Name != expect.Name {
			return true
		}
		if !reflect.DeepEqual(cur.Spec, expect.Spec) {
			l.Info("deployment change", "diff", cmp.Diff(cur.Spec, expect.Spec))
			return true
		}
		return false
	})
	if err != nil {
		return false, err
	}
	// 我们已经通过patch生成了期望的configmap，所以这里可以直接更新
	err = deleteOrCreateOrUpdate(ctx, d.Cli, d.Log, cur.Common, expect.Common, func(cur *corev1.ConfigMap, expect *corev1.ConfigMap) bool {
		change := !reflect.DeepEqual(cur.Data, expect.Data)
		if change {
			l.Info("common configmap change", "diff", cmp.Diff(cur.Data, expect.Data))
		}
		return change
	})
	if err != nil {
		return false, err
	}
	// 端口项目是完全从config生成的
	err = deleteOrCreateOrUpdate(ctx, d.Cli, d.Log, cur.PortInfo, expect.PortInfo, func(cur *corev1.ConfigMap, expect *corev1.ConfigMap) bool {
		change := !reflect.DeepEqual(cur.Data, expect.Data)
		if change {
			l.Info("port info change ", "diff", cmp.Diff(cur.Data, expect.Data))
		}
		return change
	})
	if err != nil {
		return false, err
	}

	err = deleteOrCreateOrUpdate(ctx, d.Cli, d.Log, cur.Svc.Svc, expect.Svc.Svc, func(cur *corev1.Service, expect *corev1.Service) bool {
		labelChange := !(reflect.DeepEqual(cur.Labels, expect.Labels))
		selectorChange := !(SameMap(cur.Spec.Selector, expect.Spec.Selector))
		change := labelChange || selectorChange
		if change {
			l.Info("svc change ", "diff", cmp.Diff(cur, expect))
		}
		return change
	})
	if err != nil {
		return false, err
	}

	// 容器网络模式下 svc有可能会被alb自己更新，比如lb类型的svc会自己加端口，所以不能更新
	{
		err = deleteOrCreate(ctx, d.Cli, d.Log, cur.Svc.TcpSvc, expect.Svc.TcpSvc)
		if err != nil {
			return false, err
		}
		err = deleteOrCreate(ctx, d.Cli, d.Log, cur.Svc.UdpSvc, expect.Svc.UdpSvc)
		if err != nil {
			return false, err
		}
	}
	// ingressclass中controller是immutable的,所以这里保持原样
	err = deleteOrCreate(ctx, d.Cli, d.Log, cur.Ingress, expect.Ingress)
	if err != nil {
		return false, err
	}

	// update gatewayclass
	err = deleteOrCreate(ctx, d.Cli, d.Log, cur.Gateway, expect.Gateway)
	if err != nil {
		return false, err
	}

	// update feature
	err = deleteOrCreateOrUpdate(ctx, d.Cli, d.Log, cur.Feature, expect.Feature, func(cur *unstructured.Unstructured, expect *unstructured.Unstructured) bool {
		curAddress, find, err := unstructured.NestedString(cur.Object, "spec", "accessInfo", "host")
		if err != nil {
			l.Error(err, "get address from current feature fail")
			return false
		}
		if !find {
			curAddress = ""
		}
		expectAddress, find, err := unstructured.NestedString(expect.Object, "spec", "accessInfo", "host")
		if err != nil {
			l.Error(err, "get address from expect feature fail")
			return false
		}
		if !find {
			expectAddress = ""
		}
		change := curAddress != expectAddress
		if change {
			l.Info("address change ", "cur", curAddress, "expect", expectAddress)
		}
		return change
	})

	if err != nil {
		return false, err
	}

	// alb 需要考虑status，自己做处理
	expect.Alb.Status = GenExpectStatus(d.Cfg, d.Env, cur.Alb.Status, expect.Deploy)
	if !reflect.DeepEqual(cur.Alb.Status, expect.Alb.Status) {
		l.Info("alb status update", "origin", cur.Alb.Status, "new", expect.Alb.Status)
		err := d.Cli.Status().Update(ctx, expect.Alb)
		if err != nil {
			return false, err
		}
		return false, nil
	} else {
		l.Info("alb status not change", "alb", ShowMeta(cur.Alb))
	}
	return false, nil
}

func (d *AlbDeployCtl) SameAlb(origin *albv2.ALB2, new *albv2.ALB2) (bool, string) {
	sameAnnotation := SameMap(origin.GetAnnotations(), new.GetAnnotations())
	sameLabel := SameMap(origin.GetLabels(), new.GetLabels())
	sameSpec := reflect.DeepEqual(origin.Spec, new.Spec)
	if sameAnnotation && sameLabel && sameSpec {
		return true, ""
	}
	return false, fmt.Sprintf("annotation %v label %v spec %v", sameAnnotation, sameLabel, sameSpec)
}

func SameMap(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for k, v := range left {
		rv, find := right[k]
		if !find || rv != v {
			return false
		}
	}
	return true
}

type MigrateKind string

const CreateIfNotExistKind = MigrateKind("CreateIfNotExist")
const DeleteOrUpdateOrCreateKind = MigrateKind("DeleteOrUpdateOrCreate")
const CreateOrDeleteKind = MigrateKind("CreateOrDelete")

func deleteOrCreate[T client.Object](ctx context.Context, cli client.Client, log logr.Logger, cur T, expect T) error {
	return migrateCr(CreateOrDeleteKind, ctx, cli, log, cur, expect, func(cur T, expect T) bool { return true })
}

func deleteOrCreateOrUpdate[T client.Object](ctx context.Context, cli client.Client, log logr.Logger, cur T, expect T, need func(cur, expect T) bool) error {
	return migrateCr(DeleteOrUpdateOrCreateKind, ctx, cli, log, cur, expect, need)
}

func migrateCr[T client.Object](kind MigrateKind, ctx context.Context, cli client.Client, log logr.Logger, cur T, expect T, need func(cur, expect T) bool) error {
	if !IsNil(cur) && IsNil(expect) {
		log.Info("do delete", "cur", ShowMeta(cur))
		err := cli.Delete(ctx, expect)
		if err != nil {
			return perr.Wrapf(err, "delete %v %v fail", cur.GetObjectKind(), cur.GetName())
		}
		log.Info("do delete success", "cur", ShowMeta(expect))
	}
	if IsNil(cur) && !IsNil(expect) {
		log.Info("do create", "cr", PrettyCr(expect))
		err := cli.Create(ctx, expect)
		if err != nil {
			return perr.Wrapf(err, "create %v %v fail", expect.GetObjectKind(), expect.GetName())
		}
		log.Info("do create success", "expect", ShowMeta(expect))
	}
	if kind == CreateIfNotExistKind || kind == CreateOrDeleteKind {
		log.Info("do nothing.", "kind", kind, "cur", ShowMeta(cur), "expect", ShowMeta(expect))
		return nil
	}
	if !IsNil(cur) && !IsNil(expect) {
		if !need(cur, expect) {
			log.Info("same, ignore update.", "cur", ShowMeta(cur), "expect", ShowMeta(expect))
			return nil
		}
		log.Info("not same, do update.", "cur", ShowMeta(cur), "expect", ShowMeta(expect))
		err := cli.Update(ctx, expect)
		if err != nil {
			return perr.Wrapf(err, "update %v %v fail", expect.GetObjectKind(), expect.GetName())
		}
		return nil
	}
	return nil
}
