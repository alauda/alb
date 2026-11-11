package controllers

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"slices"

	"alauda.io/alb2/pkg/operator/config"
	. "alauda.io/alb2/pkg/operator/controllers/depl/util"
	. "alauda.io/alb2/pkg/operator/controllers/types"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
	gv1 "sigs.k8s.io/gateway-api/apis/v1"
)

type Ctx struct {
	ctx         context.Context
	rest_cfg    *rest.Config
	dynamic_cli *dynamic.DynamicClient
	log         logr.Logger
}

func CtxFrom(ctx context.Context, rest_cfg *rest.Config, log logr.Logger) (*Ctx, error) {
	cli, err := dynamic.NewForConfig(rest_cfg)
	if err != nil {
		return nil, err
	}
	return &Ctx{
		ctx:         ctx,
		rest_cfg:    rest_cfg,
		dynamic_cli: cli,
		log:         log,
	}, nil
}

func (c *Ctx) gatewayClassCleanup() {
	log := c.log
	log.Info("on gatewayclass cleanup")
	gws, err := c.listAlbRelatedGateway()
	if err != nil {
		log.Error(err, "check alb gateway fail")
		return
	}
	if len(gws) != 0 {
		log.Info("find gateway still use alb", "gws", gws)
		return
	}
	log.Info("find no gateway still use alb do delete")
	c.deleteAlbGatewayClass()
}

func StandAloneGatewayClassCleanup(ctx context.Context, rest_cfg *rest.Config, log logr.Logger) {
	c, err := CtxFrom(ctx, rest_cfg, log)
	if err != nil {
		log.Error(err, "init ctx fail")
	}
	c.gatewayClassCleanup()
}

func (c *Ctx) listAlbRelatedGateway() ([]string, error) {
	ret := []string{}
	cli := c.dynamic_cli
	ctx := c.ctx
	ls, err := cli.Resource(gv1.SchemeGroupVersion.WithResource("gateways")).Namespace("").List(ctx, metav1.ListOptions{LabelSelector: "alb.cpaas.io/alb-ref"})
	if err != nil {
		return ret, err
	}
	for _, k := range ls.Items {
		ret = append(ret, fmt.Sprintf("%s/%s", k.GetNamespace(), k.GetName()))
	}
	return ret, nil
}

func (c *Ctx) deleteAlbGatewayClass() {
	name := STAND_ALONE_GATEWAY_CLASS
	log := c.log
	cli := c.dynamic_cli
	ctx := c.ctx
	c.log.Info("delete gateway class", "name", name)

	err := cli.Resource(gv1.SchemeGroupVersion.WithResource("gatewayclasses")).Delete(ctx, name, metav1.DeleteOptions{})
	if k8serrors.IsNotFound(err) {
		log.Info("gatewayclass not found ignore", "name", name)
		return
	}
	if err != nil {
		log.Error(err, "failed to delete gatewayclass", "name", name)
	}
}

func StandAloneGatewayClassInit(ctx context.Context, cfg config.OperatorCfg, cli crcli.Client, log logr.Logger) error {
	// 创建默认的独享型的gatewayclass
	labels := MergeMap(OperatorLabel(cfg.Version), map[string]string{
		fmt.Sprintf("gatewayclass.%s/deploy", cfg.BaseDomain): cfg.BaseDomain,
		fmt.Sprintf("gatewayclass.%s/type", cfg.BaseDomain):   "standalone",
	})
	gclass := &gv1.GatewayClass{}
	name := STAND_ALONE_GATEWAY_CLASS
	ctlName := gv1.GatewayController(fmt.Sprintf(FMT_STAND_ALONE_GATEWAY_CLASS_CTL_NAME, cfg.BaseDomain))
	err := cli.Get(ctx, crcli.ObjectKey{Name: name}, gclass)
	log = log.WithName("gclass")
	if k8serrors.IsNotFound(err) {
		log.Info("not found create it")
		// do create
		return cli.Create(ctx, &gv1.GatewayClass{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: labels,
			},
			Spec: gv1.GatewayClassSpec{
				ControllerName: ctlName,
			},
		})
	}
	if err != nil {
		return err
	}
	log.Info("already exist", "version", gclass.ResourceVersion)
	// update
	if gclass.Labels == nil {
		gclass.Labels = map[string]string{}
	}
	origin := gclass.DeepCopy()
	gclass.Labels = MergeMap(gclass.Labels, labels)
	gclass.Spec = gv1.GatewayClassSpec{
		ControllerName: ctlName,
	}
	if reflect.DeepEqual(origin.Labels, gclass.Labels) && reflect.DeepEqual(origin.Spec, gclass.Spec) {
		log.Info("all same ignore")
		return nil
	}
	log.Info("find diff update it", "diff", cmp.Diff(origin, gclass))
	return cli.Update(ctx, gclass)
}

func AlbDeactivate(ctx context.Context, rest_cfg *rest.Config, log logr.Logger) {
	c, err := CtxFrom(ctx, rest_cfg, log)
	if err != nil {
		log.Error(err, "init ctx fail")
	}
	c.albDeactivate()
}

func (c *Ctx) albDeactivate() {
	log := c.log
	log.Info("on alb deactivate")
	albs, err := c.listUserCreatedAlbs()
	if err != nil {
		log.Error(err, "check has user alb fail")
		return
	}
	if len(albs) > 0 {
		log.Info("find has user alb", "albs", albs)
		return
	}
	log.Info("no user alb. init featuregate")
	// only disable alb2 when no user alb
	c.setAlaudaFeatureGate("alb2", false)
}

func (c *Ctx) listUserCreatedAlbs() ([]string, error) {
	alb_list, err := c.listAlb()
	if err != nil {
		return []string{}, err
	}
	alb_map := map[string]bool{}
	for _, k := range alb_list {
		alb_map[k] = true
	}
	delete(alb_map, "cpaas-system/cpaas-system")
	delete(alb_map, "cpaas-system/global-alb2")
	return slices.Collect(maps.Keys(alb_map)), nil
}

func (c *Ctx) listAlb() ([]string, error) {
	ret := []string{}

	gvr := schema.GroupVersionResource{Group: "crd.alauda.io", Version: "v2", Resource: "alaudaloadbalancer2"}

	ls, err := c.dynamic_cli.Resource(gvr).Namespace("").List(c.ctx, metav1.ListOptions{})
	if err != nil {
		return ret, err
	}
	for _, k := range ls.Items {
		ret = append(ret, fmt.Sprintf("%s/%s", k.GetNamespace(), k.GetName()))
	}
	return ret, nil
}

const GLOBAL_NS = "cpaas-system"

// due to https://jira.alauda.cn/browse/AIT-61456. we do it manually
func (c *Ctx) setAlaudaFeatureGate(name string, flag bool) {
	cluster_gate := schema.GroupVersionResource{
		Group:    "alauda.io",
		Version:  "v1",
		Resource: "clusteralaudafeaturegates",
	}
	gate := schema.GroupVersionResource{
		Group:    "alauda.io",
		Version:  "v1",
		Resource: "alaudafeaturegates",
	}
	err := c.setAlaudaFeatureGateWithGVR(name, flag, cluster_gate)
	if err != nil {
		c.log.Error(err, "set cluster gate fail", "name", name, "flag", flag)
	}
	err = c.setAlaudaFeatureGateWithGVR(name, flag, gate)
	if err != nil {
		c.log.Error(err, "set gate fail", "name", name, "flag", flag)
	}
}

func (c *Ctx) setAlaudaFeatureGateWithGVR(name string, flag bool, gvr schema.GroupVersionResource) error {
	gate, err := c.dynamic_cli.Resource(gvr).Namespace(GLOBAL_NS).Get(c.ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		c.log.Info("set gate not found ignore", "name", name, "gvr", gvr)
		return nil
	}

	annotations := gate.GetAnnotations()
	if annotations != nil && annotations["cpaas.io/ignoreme"] == "true" {
		c.log.Info("ignore this due to annotation", "name", name, "gvr", gvr)
		return nil
	}
	origin_flag, find, err := unstructured.NestedBool(gate.Object, "spec", "enabled")
	if err != nil || !find {
		return fmt.Errorf("nestbool fail, err %v find %v", err, find)
	}
	if origin_flag == flag {
		c.log.Info("find gate same ignore", "name", name, "gvr", gvr)
		return nil
	}
	_ = gvr
	err = unstructured.SetNestedField(gate.Object, flag, "spec", "enabled")
	if err != nil {
		return fmt.Errorf("set nestbool fail, err %v", err)
	}
	_, err = c.dynamic_cli.Resource(gvr).Namespace(GLOBAL_NS).Update(c.ctx, gate, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update fail, err %v", err)
	}
	c.log.Info("set enable ok", "flag", flag, "gvr", gvr)
	return nil
}
