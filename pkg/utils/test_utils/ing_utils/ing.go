package ing_utils

import (
	"context"
	"fmt"
	"time"

	"alauda.io/alb2/config"
	ct "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	ing "alauda.io/alb2/ingress"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	. "alauda.io/alb2/pkg/utils/test_utils"
	. "alauda.io/alb2/utils/test_utils"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
)

func SyncIngressAndGetPolicyFromK8s(rest *rest.Config, ns, name string, log logr.Logger) (*ct.NgxPolicy, error) {
	ctx := context.Background()
	cfg := config.Mock(name, ns)
	drv, err := driver.NewDriver(driver.DrvOpt{Ctx: ctx, Cf: rest, Opt: driver.Cfg2opt(cfg)})
	if err != nil {
		return nil, err
	}
	ing_ctl := ing.NewController(drv, drv.Informers, cfg, log)
	ing_ctl.SyncAll()
	time.Sleep(1 * time.Second)
	go ing_ctl.RunWorker()
	for i := 0; i < 10; i++ {
		if ing_ctl.GetWorkqueueLen() == 0 {
			break
		}
		time.Sleep(1 * time.Second) // wait rule sync
	}
	time.Sleep(3 * time.Second) // wait rule sync
	policy, err := GetPolicy(PolicyGetCtx{
		Ctx: ctx, Name: name, Ns: ns, Drv: drv, L: log,
		Cfg: cfg,
	})
	if err != nil {
		return nil, err
	}
	return policy, nil
}

func GetIngressRule(kc *K8sClient, ing_ns, ing_name, rule_ns string) ([]alb2v1.Rule, error) {
	sel := labels.SelectorFromSet(labels.Set{
		"alb2.cpaas.io/source-type": "ingress",
		// "alb2.cpaas.io/source-name": ing_name,
		// "alb2.cpaas.io/source-ns":   ing_ns,
	}).String()
	rules, err := kc.GetAlbClient().CrdV1().Rules(rule_ns).List(context.Background(), metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return nil, fmt.Errorf("get rule for ingress %s/%s sel -%s- fail %s", ing_ns, ing_name, sel, err)
	}
	r := []alb2v1.Rule{}
	for _, rule := range rules.Items {
		if rule.Spec.Source.Namespace == ing_ns && rule.Spec.Source.Name == ing_name && rule.Spec.Source.Type == "ingress" {
			r = append(r, rule)
		}
	}
	return r, nil
}

func WaitIngressRule(log logr.Logger, kc *K8sClient, ns, name, rule_ns string, size int) ([]alb2v1.Rule, error) {
	rulesChan := make(chan []alb2v1.Rule, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 50*time.Second, true, wait.ConditionWithContextFunc(func(ctx context.Context) (bool, error) {
		rules, err := GetIngressRule(kc, ns, name, rule_ns)
		log.Info("wait ingress rule", "ing-ns", ns, "ing-name", name, "rule-ns", rule_ns, "size", size, "rules", len(rules), "err", err)
		if len(rules) == size {
			rulesChan <- rules
			return true, nil
		}
		return false, nil
	}))
	if err != nil {
		return nil, err
	}
	rules := <-rulesChan
	return rules, nil
}
