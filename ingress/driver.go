package ingress

import (
	"fmt"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	m "alauda.io/alb2/modules"
	"alauda.io/alb2/utils/dirhash"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// a wapper of k8s driver include method only use in ingress.
type IngressDriver struct {
	*driver.KubernetesDriver
	log logr.Logger
	config.IConfig
}

func NewDriver(d *driver.KubernetesDriver, cfg config.IConfig, log logr.Logger) *IngressDriver {
	return &IngressDriver{
		KubernetesDriver: d,
		log:              log,
		IConfig:          cfg,
	}
}

func (kd *IngressDriver) CreateFt(ft *alb2v1.Frontend) (*alb2v1.Frontend, error) {
	return kd.ALBClient.CrdV1().Frontends(ft.Namespace).Create(kd.Ctx, ft, metav1.CreateOptions{})
}

func (kd *IngressDriver) UpdateFt(ft *alb2v1.Frontend) (*alb2v1.Frontend, error) {
	ft, err := kd.ALBClient.CrdV1().Frontends(ft.Namespace).Update(kd.Ctx, ft, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	return ft, nil
}

func (kd *IngressDriver) DeleteRule(key client.ObjectKey) error {
	err := kd.ALBClient.CrdV1().Rules(key.Namespace).Delete(kd.Ctx, key.Name, metav1.DeleteOptions{})
	return err
}

func (kd *IngressDriver) CreateRule(r *alb2v1.Rule) (*alb2v1.Rule, error) {
	return kd.ALBClient.CrdV1().Rules(r.Namespace).Create(kd.Ctx, r, metav1.CreateOptions{})
}

func (kd *IngressDriver) FindIngress(key client.ObjectKey) (*networkingv1.Ingress, error) {
	return kd.Informers.K8s.Ingress.Lister().Ingresses(key.Namespace).Get(key.Name)
}

func (kd *IngressDriver) FindIngressRule() ([]*alb2v1.Rule, error) {
	sel := labels.SelectorFromSet(map[string]string{
		kd.GetLabelSourceType(): m.TypeIngress,
		kd.GetLabelAlbName():    kd.GetAlbName(),
	})
	rules, err := kd.RuleLister.Rules(kd.GetNs()).List(sel)
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (kd *IngressDriver) ListAllIngress() ([]*networkingv1.Ingress, error) {
	ings, err := kd.Informers.K8s.Ingress.Lister().Ingresses("").List(labels.Everything())
	if err != nil {
		return nil, err
	}
	return ings, nil
}

func hashSource(source *alb2v1.Source) string {
	if source == nil {
		return ""
	}
	return dirhash.LabelSafetHash(fmt.Sprintf("%s.%s", source.Name, source.Namespace))
}
