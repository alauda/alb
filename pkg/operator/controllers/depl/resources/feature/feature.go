package feature

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	a2t "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	cfg "alauda.io/alb2/pkg/operator/config"
	"alauda.io/alb2/pkg/operator/controllers/depl/util"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

type FeatureCtl struct {
	ctx    context.Context
	cli    crcli.Client
	log    logr.Logger
	enable bool
}

type FeatureCur struct {
	Raw *unstructured.Unstructured
}
type FeatureUpdate struct {
	cur    *unstructured.Unstructured
	expect *unstructured.Unstructured
}

func NewFeatureCtl(ctx context.Context, cli crcli.Client, log logr.Logger) *FeatureCtl {
	enable := HasFeatureCrd(ctx, cli)
	return &FeatureCtl{
		ctx:    ctx,
		cli:    cli,
		log:    log,
		enable: enable,
	}
}

func HasFeatureCrd(ctx context.Context, cli crcli.Client) bool {
	if cli == nil {
		return false
	}
	gk := schema.GroupKind{
		Group: "infrastructure.alauda.io",
		Kind:  "Feature",
	}
	_, err := cli.RESTMapper().RESTMapping(gk, "v1alpha1")
	return err == nil
}

func (ctl *FeatureCtl) Load(req crcli.ObjectKey) (FeatureCur, error) {
	if !ctl.enable {
		return FeatureCur{}, nil
	}

	feature := EmptyFeatureCr()
	featureKey := crcli.ObjectKey{Namespace: "", Name: fmt.Sprintf("%s-%s", req.Name, req.Namespace)}
	err := ctl.cli.Get(ctl.ctx, featureKey, feature)
	if errors.IsNotFound(err) {
		feature = nil
	}
	if err != nil && !errors.IsNotFound(err) {
		return FeatureCur{}, err
	}
	return FeatureCur{Raw: feature}, nil
}

func (ctl *FeatureCtl) GenUpdate(cur FeatureCur, cf cfg.Config, alb *a2t.ALB2) FeatureUpdate {
	if !ctl.enable {
		return FeatureUpdate{}
	}
	if !cf.ALB.Controller.Flags.EnableIngress {
		return FeatureUpdate{}
	}
	address := alb.GetAllAddress()
	return FeatureUpdate{cur: cur.Raw, expect: FeatureCr(cur.Raw, cf.ALB.Name, cf.ALB.Ns, strings.Join(address, ","))}
}

func (ctl *FeatureCtl) DoUpdate(update FeatureUpdate) error {
	if !ctl.enable {
		return nil
	}
	l := ctl.log
	err := util.DeleteOrCreateOrUpdate(ctl.ctx, ctl.cli, ctl.log, update.cur, update.expect, func(cur *unstructured.Unstructured, expect *unstructured.Unstructured) bool {
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
	return err
}

func EmptyFeatureCr() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "infrastructure.alauda.io",
		Kind:    "Feature",
		Version: "v1alpha1",
	})
	return u
}

// create a new feature base on origin.
// origin will not be modified
func FeatureCr(origin *unstructured.Unstructured, name string, ns string, host string) *unstructured.Unstructured {
	// apiVersion: infrastructure.alauda.io/v1alpha1
	// kind: Feature
	// metadata:
	//   labels:
	//     instanceType: alb2
	//     type: ingress-controller
	//   name: {{ .Values.loadbalancerName }}-{{ .Values.global.namespace }}
	// spec:
	//   accessInfo:
	//     host: {{ .Values.address | quote }}
	//   instanceType: alb2
	//   type: ingress-controller
	//   version: "1.0"
	u := &unstructured.Unstructured{}
	if origin != nil {
		u = origin.DeepCopy()
	}

	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "infrastructure.alauda.io",
		Kind:    "Feature",
		Version: "v1alpha1",
	})
	u.SetName(fmt.Sprintf("%s-%s", name, ns))
	u.SetLabels(map[string]string{
		"instanceType": "alb2",
		"type":         "ingress-controller",
	})
	_ = unstructured.SetNestedField(u.Object, map[string]interface{}{}, "spec")
	_ = unstructured.SetNestedField(u.Object, map[string]interface{}{"host": host}, "spec", "accessInfo")
	_ = unstructured.SetNestedField(u.Object, "alb2", "spec", "instanceType")
	_ = unstructured.SetNestedField(u.Object, "ingress-controller", "spec", "type")
	_ = unstructured.SetNestedField(u.Object, "1.0", "spec", "version")
	return u
}
