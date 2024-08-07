package ingress

import (
	"fmt"

	m "alauda.io/alb2/controller/modules"
	"github.com/thoas/go-funk"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Controller) GetProjectIngresses(projects []string) []*networkingv1.Ingress {
	if funk.ContainsString(projects, m.ProjectALL) {
		ingress, err := c.ingressLister.Ingresses("").List(labels.Everything())
		if err != nil {
			c.log.Error(err, "")
			return nil
		}
		return ingress
	}
	var allIngresses []*networkingv1.Ingress
	for _, project := range projects {
		sel := labels.Set{fmt.Sprintf("%s/project", c.GetDomain()): project}.AsSelector()
		nss, err := c.kd.Client.CoreV1().Namespaces().List(c.kd.Ctx, metav1.ListOptions{LabelSelector: sel.String()})
		if err != nil {
			c.log.Error(err, "")
			return nil
		}
		for _, ns := range nss.Items {
			ingress, err := c.ingressLister.Ingresses(ns.Name).List(labels.Everything())
			if err != nil {
				c.log.Error(err, "")
				return nil
			}
			allIngresses = append(allIngresses, ingress...)
		}
	}
	return allIngresses
}
