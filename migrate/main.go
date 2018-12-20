package main

import (
	"flag"
	"fmt"
	"strings"

	"alb2/config"
	"alb2/driver"
	"alb2/modules"

	crdV1 "alb2/pkg/apis/alauda/v1"

	"k8s.io/apimachinery/pkg/types"

	"github.com/golang/glog"
	v1types "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// Name is alb1 name
	Name string
	// NewNamespace is the namespace to hold alb2 related resource, default to alauda-system
	NewNamespace = "alauda-system"
	dryRun       = flag.Bool("dry-run", true, "dry run flag")
)

func main() {
	flag.Set("alsologtostderr", "true")
	flag.Parse()
	defer glog.Flush()
	ensureK8sEnv()
	k8sDriver, err := driver.GetDriver()
	if err != nil {
		panic(err)
	}
	// install necessary crd for alb2
	if err := k8sDriver.RegisterCustomDefinedResources(); err != nil {
		panic(err)
	}

	_, err = k8sDriver.Client.CoreV1().Namespaces().Get(NewNamespace, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			glog.Errorf("your specific namespace %s is not exist, to hold alb2 related resource the namespace must exist", NewNamespace)
		}
		panic(err)
	}

	alb1Resource, err := k8sDriver.ALBClient.CrdV3().AlaudaLoadBalancers().Get(Name, metav1.GetOptions{})
	if err != nil {
		panic(err)
	}

	// for migration, alb2 resources should not exist
	var domains []string
	for _, domain := range alb1Resource.Spec.Domains {
		domains = append(domains, domain.Domain)
	}
	alb2Resource := &crdV1.ALB2{
		ObjectMeta: metav1.ObjectMeta{
			Name:      alb1Resource.Name,
			Namespace: NewNamespace,
		},
		Spec: crdV1.ALB2Spec{
			Address:     alb1Resource.Spec.Address,
			BindAddress: alb1Resource.Spec.BindAddress,
			Domains:     domains,
			IaasID:      alb1Resource.Spec.IaasID,
			Type:        alb1Resource.Spec.Type,
		},
	}
	glog.Infof("will create resource alb2, %+v", alb2Resource)
	var alb2UID types.UID
	if *dryRun == false {
		albRes, err := k8sDriver.ALBClient.CrdV1().ALB2s(NewNamespace).Create(alb2Resource)
		if err != nil {
			glog.Errorf("create alb2 resource failed, %+v", err)
		}
		alb2UID = albRes.GetUID()
	}

	for _, alb1ft := range alb1Resource.Spec.Frontends {
		var ftUID types.UID
		// ref modules/alb2.go + 18
		ftName := fmt.Sprintf("%s-%d-%s", Name, alb1ft.Port, alb1ft.Protocol)
		ftResource := &crdV1.Frontend{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ftName,
				Namespace: NewNamespace,
				Labels: map[string]string{
					config.Get("labels.name"): Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					metav1.OwnerReference{
						APIVersion: crdV1.SchemeGroupVersion.String(),
						Kind:       crdV1.ALB2Kind,
						Name:       Name,
						UID:        alb2UID,
					},
				},
			},
			Spec: crdV1.FrontendSpec{
				CertificateID:   alb1ft.CertificateID,
				CertificateName: alb1ft.CertificateName,
				Port:            alb1ft.Port,
				Protocol:        alb1ft.Protocol,
			},
		}
		// frontend have default service
		if alb1ft.ServiceID != "" {
			var ftsg []crdV1.Service

			kubeSvc, err := getServiceByServiceID(k8sDriver, alb1ft.ServiceID, alb1ft.ContainerPort)
			if err != nil {
				glog.Errorf("get service by serviceID %s failed, %+v", alb1ft.ServiceID, err)
			} else if kubeSvc == nil {
				glog.Warningf("get no service by serviceID %s", alb1ft.ServiceID)
			} else {
				glog.Infof("get services %+v with serviceID %s", kubeSvc, alb1ft.ServiceID)
				ftsg = append(ftsg, crdV1.Service{
					Name:      kubeSvc.Name,
					Namespace: kubeSvc.Namespace,
					Port:      alb1ft.ContainerPort,
					Weight:    100,
				})
				ftResource.Spec.ServiceGroup = &crdV1.ServiceGroup{Services: ftsg}
			}
		}
		glog.Infof("will create resource frontend, %+v", ftResource)
		if *dryRun == false {
			ftRes, err := k8sDriver.ALBClient.CrdV1().Frontends(NewNamespace).Create(ftResource)
			if err != nil {
				glog.Errorf("create frontend resource failed, %+v", err)
			}
			ftUID = ftRes.UID
		}
		for _, alb1rule := range alb1ft.Rules {
			dsl := alb1rule.DSL
			if dsl == "" {
				dsl = modules.GetDSL(alb1rule.Domain, alb1rule.URL)

			}
			ruleResource := &crdV1.Rule{
				ObjectMeta: metav1.ObjectMeta{
					// ref modules/alb2.go + 67
					Name:      modules.RandomStr(ftName, 4),
					Namespace: NewNamespace,
					Labels: map[string]string{
						config.Get("labels.name"):     Name,
						config.Get("labels.frontend"): ftName,
					},
					OwnerReferences: []metav1.OwnerReference{
						metav1.OwnerReference{
							APIVersion: crdV1.SchemeGroupVersion.String(),
							Kind:       crdV1.FrontendKind,
							Name:       ftName,
							UID:        ftUID,
						},
					},
				},
				Spec: crdV1.RuleSpec{
					Description: alb1rule.Description,
					Domain:      alb1rule.Domain,
					DSL:         dsl,
					Priority:    alb1rule.Priority,
					Type:        alb1rule.Type,
					URL:         alb1rule.URL,
				},
			}
			if alb1rule.Services != nil {
				var rulesg []crdV1.Service
				for _, service := range alb1rule.Services {
					kubeSvc, err := getServiceByServiceID(k8sDriver, service.ServiceID, service.ContainerPort)
					if err != nil {
						glog.Errorf("get service by serviceID %s failed, %+v", service.ServiceID, err)
					} else if kubeSvc == nil {
						glog.Warningf("get no service by serviceID %s", service.ServiceID)
					} else {
						glog.Infof("get services %+v with serviceID %s", kubeSvc, service.ServiceID)
						rulesg = append(rulesg, crdV1.Service{
							Name:      kubeSvc.Name,
							Namespace: kubeSvc.Namespace,
							Port:      service.ContainerPort,
							Weight:    service.Weight,
						})
					}
				}
				ruleResource.Spec.ServiceGroup = &crdV1.ServiceGroup{
					Services:                 rulesg,
					SessionAffinityAttribute: alb1rule.SessionAffinityAttribute,
					SessionAffinityPolicy:    alb1rule.SessionAffinityPolicy,
				}
			}
			glog.Infof("will create resource rule, %+v", ruleResource)
			if *dryRun == false {
				_, err := k8sDriver.ALBClient.CrdV1().Rules(NewNamespace).Create(ruleResource)
				if err != nil {
					glog.Errorf("create rule resource failed, %+v", err)
				}
			}
		}
	}
}

func ensureK8sEnv() {
	glog.Info("KUBERNETES_SERVER: ", config.Get("KUBERNETES_SERVER"))
	glog.Info("KUBERNETES_BEARERTOKEN: ", config.Get("KUBERNETES_BEARERTOKEN"))
	glog.Info("NAME: ", config.Get("NAME"))
	if strings.TrimSpace(config.Get("KUBERNETES_SERVER")) == "" ||
		strings.TrimSpace(config.Get("KUBERNETES_BEARERTOKEN")) == "" ||
		strings.TrimSpace(config.Get("NAME")) == "" {
		panic("you must set KUBERNETES_SERVER and KUBERNETES_BEARERTOKEN and NAME env")
	}

	if strings.TrimSpace(config.Get("NEW_NAMESPACE")) != "" {
		NewNamespace = config.Get("NEW_NAMESPACE")
	}
	Name = config.Get("NAME")

	config.Set("LABEL_SERVICE_ID", "service.alauda.io/uuid")
	config.Set("LABEL_SERVICE_NAME", "service.alauda.io/name")
	config.Set("LABEL_CREATOR", "service.alauda.io/createby")

}

func getServiceByServiceID(k8sDriver *driver.KubernetesDriver, serviceID string, servicePort int) (*v1types.Service, error) {
	labelSelector := fmt.Sprintf("%s=%s", config.Get("LABEL_SERVICE_ID"), serviceID)
	services, err := k8sDriver.Client.CoreV1().Services("").List(metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}
	var kubeSvc *v1types.Service
svcLoop:
	for _, service := range services.Items {
		for _, port := range service.Spec.Ports {
			if servicePort == int(port.Port) {
				kubeSvc = &service
				if service.Labels[config.Get("LABEL_CREATOR")] == "" {
					break svcLoop
				}
			}
		}
	}
	return kubeSvc, nil
}
