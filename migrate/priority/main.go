package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"alauda.io/alb2/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

var (
	Name      string
	Namespace string
	Domain    string
	dryRun    = flag.Bool("dry-run", true, "dry run flag")
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()
	ensureEnv()
	k8sDriver, err := driver.GetDriver(context.Background())
	if err != nil {
		panic(err)
	}
	allRules, err := k8sDriver.ALBClient.CrdV1().Rules(Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("alb2.%s/name=%s", Domain, Name),
	})
	if err != nil {
		panic(err)
	}
	for _, rl := range allRules.Items {
		if rl.Spec.Priority != 0 {
			continue
		}
		rl.Spec.Priority = 5
		klog.Infof("convert rule %s/%s priority: %d", rl.Namespace, rl.Name, rl.Spec.Priority)
		if !*dryRun {
			if _, err = k8sDriver.ALBClient.CrdV1().Rules(Namespace).Update(context.TODO(), &rl, metav1.UpdateOptions{}); err != nil {
				klog.Error(err)
			}
		}
	}
}

func ensureEnv() {
	Name = os.Getenv("NAME")
	Namespace = os.Getenv("NAMESPACE")
	Domain = os.Getenv("DOMAIN")
	klog.Info("NAME: ", Name)
	klog.Info("NAMESPACE: ", Namespace)
	klog.Info("DOMAIN: ", Domain)
	if strings.TrimSpace(Name) == "" &&
		strings.TrimSpace(Namespace) == "" &&
		strings.TrimSpace(Domain) == "" {
		panic("you must set NAME and NAMESPACE and DOMAIN env")
	}
}
