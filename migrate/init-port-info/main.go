package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"alauda.io/alb2/config"
	"alauda.io/alb2/driver"
	albv1 "alauda.io/alb2/pkg/apis/alauda/v1"
	albclient "alauda.io/alb2/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

var (
	Namespace  string
	Domain     string
	dryRun     = flag.Bool("dry-run", false, "dry run flag")
	kubeConfig = flag.String("kubeconfig", "", "(optional) absolute path to the kubeconfig file")
)

type PortRangeList []PortRange

type PortRange struct {
	Port     string   `json:"port"`
	Projects []string `json:"projects"`
}

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()
	ensureEnv()
	err := run()
	if err != nil {
		panic(err)
	}
}

func ensureEnv() {
	klog.Info("NAMESPACE: ", config.Get("NAMESPACE"))
	klog.Info("DOMAIN: ", config.Get("DOMAIN"))
	if strings.TrimSpace(config.Get("NAMESPACE")) == "" ||
		strings.TrimSpace(config.Get("DOMAIN")) == "" {
		panic("you must set NAMESPACE and DOMAIN env")
	}

	Namespace = config.Get("NAMESPACE")
	Domain = config.Get("DOMAIN")
}

func run() error {
	drv, err := getDriver()
	if err != nil {
		return err
	}
	albs, err := drv.ALBClient.CrdV1().ALB2s(Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s/role=port", Domain),
	})
	if err != nil {
		return err
	}

	for _, alb := range albs.Items {
		err = updatePeerAlb(drv, alb)
		if err != nil {
			return err
		}

	}
	return nil
}

func updatePeerAlb(drv *driver.KubernetesDriver, alb albv1.ALB2) error {
	klog.Infof("port mode alb: %v", alb.Name)
	fts, err := drv.ALBClient.CrdV1().Frontends(Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("alb2.%s/name=%s", Domain, alb.Name),
	})
	if err != nil {
		return err
	}
	ports := []int{}
	for _, ft := range fts.Items {
		ports = append(ports, ft.Spec.Port)
	}
	klog.Infof("ft of alb %v: %v", alb.Name, ports)

	portRange, err := getPortInfo(drv, Namespace, fmt.Sprintf("%s-port-info", alb.Name))

	if k8serrors.IsNotFound(err) {
		portRange = []PortRange{}
		for _, port := range ports {
			portRange = append(portRange, PortRange{Port: fmt.Sprintf("%d", port), Projects: []string{"ALL_ALL"}})
		}

		err := setPortInfo(drv, Namespace, fmt.Sprintf("%s-port-info", alb.Name), portRange)
		if err != nil {
			return err
		}
		return nil
	}

	klog.Infof("exist port info of %v: %v", alb.Name, portRange)
	needUpdate, portRange, err := gerneratePortRange(ports, portRange)
	if err != nil {
		return err
	}
	if needUpdate {
		err := setPortInfo(drv, Namespace, fmt.Sprintf("%s-port-info", alb.Name), portRange)
		if err != nil {
			return err
		}
	} else {
		klog.Infof("not need update")
	}

	return nil
}

func strContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func gerneratePortRange(ports []int, portRange []PortRange) (needUpdate bool, retportRange []PortRange, err error) {
	needUpdate = false
	for _, port := range ports {
		hasPort := false
		for index, pt := range portRange {
			contains, err := portRangeContains(pt.Port, port)
			if err != nil {
				return false, nil, fmt.Errorf("check port contains fail %v", err)
			}
			if contains {
				if strContains(pt.Projects, "ALL_ALL") {
					// ignore this port
					hasPort = true
					break
				} else {
					needUpdate = true
					hasPort = true
					// add project
					portRange[index].Projects = append(portRange[index].Projects, "ALL_ALL")
					break
				}
			}
		}
		if !hasPort {
			needUpdate = true
			portRange = append(portRange, PortRange{Port: fmt.Sprintf("%d", port), Projects: []string{"ALL_ALL"}})
		}
	}
	return needUpdate, portRange, nil
}

func portRangeContains(portRange string, port int) (bool, error) {
	ports := strings.Split(portRange, "-")
	if len(ports) == 2 {
		start, err := strconv.Atoi(ports[0])
		if err != nil {
			return false, err
		}
		end, err := strconv.Atoi(ports[1])
		if err != nil {
			return false, err
		}

		if port >= start && port <= end {
			return true, nil
		} else {
			return false, nil
		}
	} else if len(ports) == 1 {
		originPort, err := strconv.Atoi(ports[0])
		if err != nil {
			return false, err
		}
		return originPort == port, nil

	}
	return false, fmt.Errorf("invalid format of")
}

func getDriver() (*driver.KubernetesDriver, error) {
	if *kubeConfig != "" {
		return drvFromLocalConfig(*kubeConfig)
	}
	return driver.GetDriver()
}

func drvFromLocalConfig(configPath string) (*driver.KubernetesDriver, error) {
	cf, err := clientcmd.BuildConfigFromFlags("", *kubeConfig)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(cf)
	if err != nil {
		return nil, err
	}
	albClient, err := albclient.NewForConfig(cf)
	if err != nil {
		return nil, err
	}
	dynamicClient, err := dynamic.NewForConfig(cf)
	if err != nil {
		return nil, err
	}
	return &driver.KubernetesDriver{Client: client, ALBClient: albClient, DynamicClient: dynamicClient}, nil
}

func getPortInfo(drv *driver.KubernetesDriver, ns, name string) ([]PortRange, error) {
	cf, err := drv.Client.CoreV1().ConfigMaps(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if _, ok := cf.Data["range"]; !ok {
		return nil, fmt.Errorf("could not find range in configmap %v", cf.Name)
	}
	var portRange PortRangeList
	err = json.Unmarshal([]byte(cf.Data["range"]), &portRange)
	if err != nil {
		return nil, err
	}
	return portRange, err
}

func setPortInfo(drv *driver.KubernetesDriver, ns, name string, portRange []PortRange) error {
	cf, err := drv.Client.CoreV1().ConfigMaps(ns).Get(context.TODO(), name, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		klog.Infof("create port-info %v %v", name, portRange)
		if !*dryRun {
			klog.Info("create it")
			_, err = drv.Client.CoreV1().ConfigMaps(ns).Create(context.TODO(), &corev1.ConfigMap{}, metav1.CreateOptions{})
			return err
		}
	}
	klog.Infof("update port-info %v %v", name, portRange)
	rangeJsonStr, err := json.Marshal(portRange)
	if err != nil {
		return err
	}
	if !*dryRun {
		cf.Data["range"] = string(rangeJsonStr)
		klog.Info("update it")
		_, err = drv.Client.CoreV1().ConfigMaps(ns).Update(context.TODO(), cf, metav1.UpdateOptions{})
	}

	return err
}
