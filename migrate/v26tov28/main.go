package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"alauda.io/alb2/driver"
	"alauda.io/alb2/utils"
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
	k8sDriver, err := driver.GetDriver(context.TODO())
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
		if rl.Spec.DSLX != nil {
			continue
		}
		dslx, err := utils.DSL2DSLX(rl.Spec.DSL)
		if err != nil {
			klog.Warningf("failed to convert rule %s/%s dsl %s to dslx", rl.Namespace, rl.Name, rl.Spec.DSL)
		}
		rl.Spec.DSLX = dslx
		klog.Infof("convert rule %s/%s dsl: %s to dslx: %+v", rl.Namespace, rl.Name, rl.Spec.DSL, dslx)
		if *dryRun == false {
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
