package main

import (
	"flag"
	"fmt"
	"strings"

	"alb2/config"
	"alb2/driver"
	"alb2/modules"

	crdV1 "alb2/pkg/apis/alauda/v1"

	"github.com/golang/glog"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	Name string
	// NewNamespace is the namespace to hold alb2 related resource, default to alauda-system
	Namespace = "alauda-system"
	dryRun    = flag.Bool("dry-run", true, "dry run flag")
)

func main() {
	flag.Set("alsologtostderr", "true")
	flag.Parse()
	defer glog.Flush()
	ensureK8sEnv()
	albClient, err := driver.GetALBClient()
	if err != nil {
		panic(err)
	}
	k8sClient, err := driver.GetDriver()
	if err != nil {
		panic(err)
	}

	_, err = k8sClient.Client.CoreV1().Namespaces().Get(Namespace, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			glog.Errorf("your specific namespace %s is not exist", Namespace)
		}
		panic(err)
	}

	alb1Resource, err := albClient.CrdV3().AlaudaLoadBalancers().Get(Name, metav1.GetOptions{})
	if err != nil {
		panic(err)
	}

	// for migration, alb2 resources should not exist
	var domains []string
	for _, domain := range alb1Resource.Spec.Domains {
		domains = append(domains, domain.Domain)
	}
	alb2Resource := &crdV1.AlaudaLoadBalancer2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alb1Resource.Name,
			Namespace: Namespace,
		},
		Spec: crdV1.AlaudaLoadBalancer2Spec{
			Address:     alb1Resource.Spec.Address,
			BindAddress: alb1Resource.Spec.BindAddress,
			Domains:     domains,
			IaasID:      alb1Resource.Spec.IaasID,
			Type:        alb1Resource.Spec.Type,
		},
	}
	glog.Infof("will create resource alb2, %+v", alb2Resource)
	if *dryRun == false {
		_, err := albClient.CrdV1().AlaudaLoadBalancer2s(Namespace).Create(alb2Resource)
		if err != nil {
			glog.Errorf("create alb2 resource failed, %+v", err)
		}
	}

	for _, alb1ft := range alb1Resource.Spec.Frontends {
		// ref modules/alb2.go + 18
		ftName := fmt.Sprintf("%s-%d-%s", Name, alb1ft.Port, alb1ft.Protocol)
		// TODO:
		var ftsg []crdV1.Service
		ftResource := &crdV1.Frontend{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ftName,
				Namespace: Namespace,
			},
			Spec: crdV1.FrontendSpec{
				CertificateID:   alb1ft.CertificateID,
				CertificateName: alb1ft.CertificateName,
				Port:            alb1ft.Port,
				Protocol:        alb1ft.Protocol,
				ServiceGroup: &crdV1.ServiceGroup{
					Services: ftsg,
				},
			},
		}
		glog.Infof("will create resource frontend, %+v", ftResource)
		if *dryRun == false {
			_, err := albClient.CrdV1().Frontends(Namespace).Create(ftResource)
			if err != nil {
				glog.Errorf("create frontend resource failed, %+v", err)
			}
		}
		for _, alb1rule := range alb1ft.Rules {
			// TODO:
			var rulesg []crdV1.Service
			ruleResource := &crdV1.Rule{
				ObjectMeta: metav1.ObjectMeta{
					// ref modules/alb2.go + 67
					Name:      modules.RandomStr(ftName, 4),
					Namespace: Namespace,
				},
				Spec: crdV1.RuleSpec{
					Description: alb1rule.Description,
					Domain:      alb1rule.Domain,
					DSL:         alb1rule.DSL,
					Priority:    alb1rule.Priority,
					ServiceGroup: &crdV1.ServiceGroup{
						Services: rulesg,
					},
					Type: alb1rule.Type,
					URL:  alb1rule.URL,
				},
			}
			glog.Infof("will create resource rule, %+v", ruleResource)
			if *dryRun == false {
				_, err := albClient.CrdV1().Rules(Namespace).Create(ruleResource)
				if err != nil {
					glog.Errorf("create rule resource failed, %+v", err)
				}
			}
		}
	}
}

func ensureK8sEnv() {
	if strings.TrimSpace(config.Get("KUBERNETES_SERVER")) == "" ||
		strings.TrimSpace(config.Get("KUBERNETES_BEARERTOKEN")) == "" ||
		strings.TrimSpace(config.Get("NAME")) == "" ||
		strings.TrimSpace(config.Get("NAMESPACE")) == "" {
		panic("you must set KUBERNETES_SERVER and KUBERNETES_BEARERTOKEN and NAME and NAMESPACE env")
	}
	Name = config.Get("NAME")
	Namespace = config.Get("NAMESPACE")
}
