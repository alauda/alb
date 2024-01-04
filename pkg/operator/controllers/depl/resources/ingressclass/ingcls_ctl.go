package ingressclass

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	pcfg "alauda.io/alb2/pkg/config"
	cfg "alauda.io/alb2/pkg/operator/config"
	. "alauda.io/alb2/pkg/operator/controllers/depl/util"
	. "alauda.io/alb2/pkg/operator/toolkit"
	mapset "github.com/deckarep/golang-set/v2"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IngClsCtl struct{}

func NewIngClsCtl() *IngClsCtl {
	return &IngClsCtl{}
}

const (
	INGCLS_PROJECT_FMT = "alb.%s/project"
	ALL_PROJECT        = "ALL_ALL"
)

func (c *IngClsCtl) GenExpectIngressClass(origin *netv1.IngressClass, conf *cfg.Config) (*netv1.IngressClass, error) {
	var (
		ns              = conf.ALB.Ns
		name            = conf.ALB.Name
		refLabel        = ALB2ResourceLabel(ns, name, conf.Operator.Version)
		labelBaseDomain = conf.Operator.BaseDomain
	)
	flags := conf.ALB.Controller.Flags
	if !flags.EnableIngress {
		return nil, nil
	}

	ic := origin
	if ic == nil {
		ic = &netv1.IngressClass{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},

			Spec: netv1.IngressClassSpec{
				Controller: FmtKeyBySep("/", labelBaseDomain, "alb2"),
			},
		}
	}
	ic.Labels = MergeMap(ic.Labels, refLabel)
	ic.Annotations = MergeMap(ic.Annotations, map[string]string{
		"ingressclass.kubernetes.io/is-default-class":    strconv.FormatBool(conf.ALB.Flags.DefaultIngressClass),
		fmt.Sprintf(INGCLS_PROJECT_FMT, labelBaseDomain): strings.Join(c.genProjects(conf), ","),
	})

	return ic, nil
}

func (c *IngClsCtl) genProjects(conf *cfg.Config) []string {
	if !conf.ALB.Project.EnablePortProject {
		return conf.ALB.Project.Projects
	}
	projects := pcfg.PortProject{}
	err := json.Unmarshal([]byte(conf.ALB.Project.PortProjects), &projects)
	if err != nil {
		return []string{}
	}
	httpProjectset := mapset.NewSet(getProjectByPort(conf.ALB.Controller.HttpPort, projects)...)
	httpsProjectset := mapset.NewSet(getProjectByPort(conf.ALB.Controller.HttpsPort, projects)...)

	if httpProjectset.Equal(httpsProjectset) {
		return httpProjectset.ToSlice()
	}
	if httpProjectset.Contains(ALL_PROJECT) {
		return httpsProjectset.ToSlice()
	}
	if httpsProjectset.Contains(ALL_PROJECT) {
		return httpProjectset.ToSlice()
	}
	ret := httpProjectset.Intersect(httpsProjectset).ToSlice()
	return ret
}

func getProjectByPort(port int, portprojects pcfg.PortProject) []string {
	for _, pp := range portprojects {
		if pp.Port == fmt.Sprintf("%d", port) {
			return pp.Projects
		}
		if !strings.Contains(pp.Port, "-") {
			continue
		}

		ports := strings.Split(pp.Port, "-")
		if len(ports) != 2 {
			continue
		}
		start, err := strconv.Atoi(ports[0])
		if err != nil {
			continue
		}
		end, err := strconv.Atoi(ports[1])
		if err != nil {
			continue
		}
		if port >= start && port <= end {
			return pp.Projects
		}
	}
	return []string{}
}
