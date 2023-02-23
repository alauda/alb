package controller

import (
	"context"
	"fmt"

	"alauda.io/alb2/config"
	. "alauda.io/alb2/controller/types"
	"alauda.io/alb2/driver"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO move to ft reconcile
// in container mode, we want to create/update loadbalancer tcp/udp service,use it as high avaiable solution.
func (nc *NginxController) SyncLbSvcPort(frontends []*Frontend) error {
	log := nc.log.WithName("svc_sync")
	log.Info("sync service")
	drv := nc.Driver
	ns := config.GetNs()
	albName := config.GetAlbName()

	albServices, err := nc.getCurrentServices(drv, ns, albName)
	if err != nil {
		return err
	}

	for albService, serviceProtocol := range albServices {
		serviceProtocolFrontends := nc.filterProtocolFrontends(serviceProtocol, frontends)
		need, err := nc.checkNeedUpdateALBService(albService, serviceProtocol, serviceProtocolFrontends)
		if err != nil {
			return err
		}

		log.V(5).Info("need sync", "need", need)
		if !need {
			continue
		}
		log.Info("sync ft port for alb2 service.", "ports", albService.Spec.Ports, "need", need, "ns", ns, "name", albService.Name)
		_, err = drv.Client.CoreV1().Services(ns).Update(context.TODO(), albService, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error happens when sync ft port for alb2 service %v/%v, reason: %v", ns, albService.Name, err.Error())
		}
	}
	return nil
}

func (nc *NginxController) getCurrentServices(drv *driver.KubernetesDriver, ns string, albName string) (map[*corev1.Service]corev1.Protocol, error) {
	albServices := make(map[*corev1.Service]corev1.Protocol)
	albServiceNames := make(map[string]corev1.Protocol)

	if nc.albcfg.GetNetworkMode() == config.Container {
		albServiceNames[fmt.Sprintf("%v-%v", albName, "tcp")] = corev1.ProtocolTCP
		albServiceNames[fmt.Sprintf("%v-%v", albName, "udp")] = corev1.ProtocolUDP
	} else {
		albServiceNames[albName] = ""
	}

	for serviceName, serviceProtocol := range albServiceNames {
		albService, err := drv.Client.CoreV1().Services(ns).Get(context.TODO(), serviceName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error happens when get alb2 service %v/%v, reason: %v", ns, serviceName, err.Error())
		} else {
			albServices[albService] = serviceProtocol
		}
	}
	return albServices, nil
}

// if will change spec.port in albService if need.
func (nc *NginxController) checkNeedUpdateALBService(albService *corev1.Service, serviceProtocol corev1.Protocol, frontends []*Frontend) (bool, error) {
	var needUpdate bool
	var metricsPortFlag bool
	portsList := make(map[int32]struct{})
	metricsPorts, err := nc.getMetricsPort(serviceProtocol)
	if err != nil {
		return false, err
	}

	for _, port := range albService.Spec.Ports {
		if port.Name != "metrics" {
			portsList[port.Port] = struct{}{}
		} else {
			metricsPortFlag = true
		}
	}
	if len(portsList) != len(frontends) {
		needUpdate = true
	}
	if !metricsPortFlag && metricsPorts != nil {
		needUpdate = true
	}

	albService.Spec.Ports = metricsPorts

	for _, ft := range frontends {
		_, ok := Ft2SvcProtocolMap[ft.Protocol]
		if !ok {
			return false, fmt.Errorf("frontend port %v, spec.protocol is invalid as value %v", ft.Port, ft.Protocol)
		}
		albService.Spec.Ports = append(albService.Spec.Ports, corev1.ServicePort{Name: fmt.Sprintf("%v-%v", ft.Protocol, ft.Port), Protocol: serviceProtocol, Port: int32(ft.Port), NodePort: 0})
		if !needUpdate {
			if _, portExist := portsList[int32(ft.Port)]; !portExist {
				needUpdate = true
			}
		}
	}
	return needUpdate, nil
}

func (nc *NginxController) getMetricsPort(serviceProtocol corev1.Protocol) ([]corev1.ServicePort, error) {
	var metricsPorts []corev1.ServicePort
	metricsPort := int32(config.GetInt("METRICS_PORT"))
	if metricsPort > 0 && metricsPort <= 65535 {
		metricsPorts = []corev1.ServicePort{{Name: "metrics", Port: metricsPort, Protocol: serviceProtocol}}
	} else {
		return nil, fmt.Errorf("ENV parameter METRICS_PORT is wrong value as(should be in 1-65535): %v", metricsPort)
	}
	return metricsPorts, nil
}

// LoadBalancer Service could only have one protocol.
var Ft2SvcProtocolMap = map[albv1.FtProtocol]apiv1.Protocol{
	albv1.FtProtocolHTTP:  apiv1.ProtocolTCP,
	albv1.FtProtocolHTTPS: apiv1.ProtocolTCP,
	albv1.FtProtocolgRPC:  apiv1.ProtocolTCP,
	albv1.FtProtocolTCP:   apiv1.ProtocolTCP,
	albv1.FtProtocolUDP:   apiv1.ProtocolUDP,
}

func (nc *NginxController) filterProtocolFrontends(serviceProtocol corev1.Protocol, frontends []*Frontend) []*Frontend {
	var filteredFrontends []*Frontend
	for _, frontend := range frontends {
		if Ft2SvcProtocolMap[frontend.Protocol] == serviceProtocol {
			filteredFrontends = append(filteredFrontends, frontend)
		}
	}
	return filteredFrontends
}
