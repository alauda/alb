package depl

import (
	"context"
	"fmt"
	"reflect"
	"strings"

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
	gv1b1t "sigs.k8s.io/gateway-api/apis/v1beta1"

	perr "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// 当前集群上alb相关的cr
type AlbDeploy struct {
	Alb      *albv2.ALB2
	Deploy   *appsv1.Deployment
	Common   *corev1.ConfigMap
	PortInfo *corev1.ConfigMap
	Ingress  *netv1.IngressClass
	Gateway  *gv1b1t.GatewayClass
	Svc      service.CurSvc
	Feature  *unstructured.Unstructured
}

// 为了完成升级alb所需的一些配置
type AlbDeployUpdate struct {
	Alb      *albv2.ALB2
	Deploy   *appsv1.Deployment
	Common   *corev1.ConfigMap
	PortInfo *corev1.ConfigMap
	Ingress  *netv1.IngressClass
	Gateway  *gv1b1t.GatewayClass
	Svc      service.SvcUpdates // 目前只有service有自己的配置
	Feature  *unstructured.Unstructured
}

func (d *AlbDeploy) Show() string {
	return fmt.Sprintf("alb %v,depl %v,comm %v,port %v,ic %v,gc %v,svc %v",
		showCr(d.Alb),
		showCr(d.Deploy),
		showCr(d.Common),
		showCr(d.PortInfo),
		showCr(d.Ingress),
		showCr(d.Gateway),
		d.Svc.Show())
}

func showCr(obj client.Object) string {
	if IsNil(obj) {
		return "isnil"
	}
	return fmt.Sprintf("name %v kind %v version %v", obj.GetName(), obj.GetObjectKind().GroupVersionKind().String(), obj.GetResourceVersion())
}

type AlbDeployCtl struct {
	Cli    *resourceclient.ALB2ResourceClient
	Cfg    cfg.Config
	Log    logr.Logger
	SvcCtl *service.SvcCtl
}

func NewAlbDeployCtl(ctx context.Context, cli ctlClient.Client, cfg cfg.Config, log logr.Logger) AlbDeployCtl {
	return AlbDeployCtl{
		Cli:    resourceclient.NewALB2ResourceClient(cli),
		Cfg:    cfg,
		Log:    log,
		SvcCtl: service.NewSvcCtl(ctx, cli, log, cfg.Operator),
	}
}

func (d *AlbDeployCtl) GenExpectAlbDeploy(ctx context.Context, cur *AlbDeploy) (*AlbDeployUpdate, error) {
	var err error
	cfg := &d.Cfg.ALB
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
	svcupdate := d.SvcCtl.GenUpdate(cur.Svc, d.Cfg, cur.Alb)
	expect := AlbDeployUpdate{
		Alb:      alb,
		Deploy:   depl,
		Common:   comm,
		PortInfo: port,
		Ingress:  ic,
		Gateway:  gc,
		Svc:      svcupdate,
		Feature:  feature,
	}
	return &expect, nil
}

func (d *AlbDeployCtl) genExpectAlb(cur *AlbDeploy, conf *cfg.ALB2Config) (*albv2.ALB2, error) {
	if cur.Alb == nil {
		return nil, fmt.Errorf("could not find alb ")
	}
	var (
		env             = d.Cfg.Operator
		labelBaseDomain = env.GetLabelBaseDomain()
	)
	alb := cur.Alb.DeepCopy()
	var projectLabel = map[string]string{}

	projectPrefix := fmt.Sprintf("project.%s", labelBaseDomain)
	for _, project := range conf.Project.Projects {
		key := fmt.Sprintf("%s/%s", projectPrefix, project)
		projectLabel[key] = "true"
	}

	alb.Labels = resources.MergeMap(projectLabel, resources.RemovePrefixKey(alb.Labels, projectPrefix))

	key := fmt.Sprintf("%s/%s", labelBaseDomain, "role")
	if conf.Project.EnablePortProject {
		alb.Labels[key] = "port"
	} else {
		delete(alb.Labels, key)
	}
	return alb, nil
}

func (d *AlbDeployCtl) genExpectDeployment(cur *AlbDeploy, conf *cfg.ALB2Config) (*appsv1.Deployment, error) {
	deploy := workload.NewTemplate(cur.Alb, cur.Deploy.DeepCopy(), conf, d.Cfg.Operator, d.Log).Generate()
	return deploy, nil
}

func (d *AlbDeployCtl) genExpectIngressClass(cur *AlbDeploy, conf *cfg.ALB2Config) (*netv1.IngressClass, error) {
	// TODO ingress class 的controller是不可变的
	var (
		alb2            = cur.Alb
		refLabel        = resources.ALB2ResourceLabel(alb2.Namespace, alb2.Name, d.Cfg.Operator.Version)
		labelBaseDomain = d.Cfg.Operator.GetLabelBaseDomain()
	)
	if !conf.Controller.Flags.EnableIngress {
		return nil, nil
	}
	ic := ingress.NewTemplate(alb2.GetNamespace(), alb2.Name, labelBaseDomain).Generate(
		ingress.AddLabel(refLabel),
	)
	return ic, nil
}

func (d *AlbDeployCtl) genExpectGatewayClass(cur *AlbDeploy, conf *cfg.ALB2Config) (*gv1b1t.GatewayClass, error) {
	var (
		alb2            = cur.Alb
		refLabel        = resources.ALB2ResourceLabel(alb2.Namespace, alb2.Name, d.Cfg.Operator.Version)
		labelBaseDomain = d.Cfg.Operator.GetLabelBaseDomain()
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
		refLabel  = resources.ALB2ResourceLabel(cur.Alb.Namespace, cur.Alb.Name, d.Cfg.Operator.Version)
		alb2      = cur.Alb
	)

	defaultPatches := []configmap.Option{
		configmap.SetOwnerRefs(ownerRefs),
		configmap.WithBindNIC(bindNic),
		configmap.AddLabel(refLabel),
	}
	// 当有patch的时候，我们要用patch
	hasPatch, overwriteConfigmap, err := patch.FindConfigmapPatch(ctx, d.Cli, conf, d.Cfg.Operator)
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
		refLabel  = resources.ALB2ResourceLabel(alb2.Namespace, alb2.Name, d.Cfg.Operator.Version)
		ownerRefs = MakeOwnerRefs(alb2)
	)
	data := conf.Project.PortProjects
	portMap := portinfo.NewTemplate(alb2.Namespace, alb2.Name, string(data)).Generate(
		portinfo.AddLabel(refLabel),
		portinfo.SetOwnerRefs(ownerRefs),
	)
	return portMap, nil
}

func (d *AlbDeployCtl) genExpectFeature(cur *AlbDeploy, conf *cfg.ALB2Config) *unstructured.Unstructured {
	if !conf.Controller.Flags.EnableIngress {
		return nil
	}
	address := strings.Split(cur.Alb.Spec.Address, ",")
	address = append(address, cur.Alb.Status.Detail.AddressStatus.Ipv4...)
	address = append(address, cur.Alb.Status.Detail.AddressStatus.Ipv6...)
	d.Log.Info("genExpectFeature", "address", address)
	return FeatureCr(cur.Feature, conf.Name, conf.Ns, strings.Join(address, ","))
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
	objs = append(objs, cur.Svc.GetObjs()...)
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
func (d *AlbDeployCtl) DoUpdate(ctx context.Context, cur *AlbDeploy, expect *AlbDeployUpdate) (reconcile bool, err error) {
	// TODO 这个函数中需要特殊处理的和基本可以忽略的混在一起了 有点乱
	l := d.Log

	// deployment的更新会导致alb的status的更新导致alb的resourceversion的变化,导致这里的当前的alb不是最新的了，所以这里要先更新alb
	// 如果alb需要更新 可能是label或者annotation的变化 我们不会去改spec的内容，那个应该是只由用户操作的
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
		update, reason := workload.NeedUpdate(cur, expect, l)
		if update {
			l.Info("deployment change", "reason", reason, "diff", cmp.Diff(cur.Spec, expect.Spec))
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
	// 更新svc
	err = d.SvcCtl.DoUpdate(expect.Svc)
	if err != nil {
		return false, err
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
			l.Info("feature address change ", "cur", curAddress, "new", expectAddress)
		}
		return change
	})

	if err != nil {
		return false, err
	}

	// alb 需要考虑status，自己做处理
	expect.Alb.Status = GenExpectStatus(d.Cfg, cur)
	if !SameStatus(cur.Alb.Status, expect.Alb.Status) {
		err := d.Cli.Status().Update(ctx, expect.Alb)
		if err != nil {
			return false, err
		}
		l.Info("alb status update", "diff", cmp.Diff(cur.Alb.Status, expect.Alb.Status))
		// 更新status时不应该在reconcile了，否则每次的probetime都是不一样的，会无限循环了
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
		err := cli.Update(ctx, expect)
		if err != nil {
			return perr.Wrapf(err, "update %v %v fail", expect.GetObjectKind(), expect.GetName())
		}
		log.Info("not same, do update.", "cur", ShowMeta(cur), "expect", ShowMeta(expect))

		return nil
	}
	return nil
}
