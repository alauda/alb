package ingress

import (
	"fmt"

	"alauda.io/alb2/config"
	ctl "alauda.io/alb2/controller"
	m "alauda.io/alb2/controller/modules"
	"alauda.io/alb2/driver"
	"github.com/thoas/go-funk"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// 给定一个ingress和一个alb 判断这个alb是否应该控制这个ingress
type IngressSelect struct {
	cfg IngressSelectOpt
	drv *driver.KubernetesDriver
}

type IngressSelectOpt struct {
	HttpPort  int
	HttpsPort int
	Domain    string
	Name      string
}

func Cfg2IngressSelectOpt(cfg *config.Config) IngressSelectOpt {
	return IngressSelectOpt{
		HttpPort:  cfg.GetIngressHttpPort(),
		HttpsPort: cfg.GetIngressHttpsPort(),
		Domain:    cfg.GetDomain(),
		Name:      cfg.Name,
	}
}

func NewIngressSelect(cfg IngressSelectOpt, drv *driver.KubernetesDriver) IngressSelect {
	return IngressSelect{
		cfg: cfg,
		drv: drv,
	}
}

func (s IngressSelect) ShouldHandleIngress(alb *m.AlaudaLoadBalancer, ing *networkingv1.Ingress) (rv bool, reason string) {
	domain := s.cfg.Domain
	IngressHTTPPort := s.cfg.HttpPort
	IngressHTTPSPort := s.cfg.HttpsPort
	if !s.CheckShouldHandleViaIngressClass(ing) {
		reason = fmt.Sprintf("ingressclass is not our %v", s.cfg.Name)
		return false, reason
	}

	belongProject := s.GetIngressBelongProject(ing)
	role := ctl.GetAlbRoleType(alb.Labels, s.cfg.Domain)
	if role == ctl.RolePort {
		hasHTTPPort := false
		hasHTTPSPort := false
		httpPortProjects := []string{}
		httpsPortProjects := []string{}
		for _, ft := range alb.Frontends {
			if ft.SamePort(IngressHTTPPort) {
				hasHTTPPort = true
				httpPortProjects = ctl.GetOwnProjectsFromLabel(ft.Name, ft.Labels, domain)
			} else if ft.SamePort(IngressHTTPSPort) {
				hasHTTPSPort = true
				httpsPortProjects = ctl.GetOwnProjectsFromLabel(ft.Name, ft.Labels, domain)
			}
			if hasHTTPSPort && hasHTTPPort {
				break
			}
		}
		// for role=port alb user should create http and https ports before using ingress
		if !(hasHTTPPort && hasHTTPSPort) {
			reason = fmt.Sprintf("role port must have both http and https port, http %v %v, https %v %v", IngressHTTPPort, hasHTTPPort, IngressHTTPSPort, hasHTTPSPort)
			return false, reason
		}
		if (funk.Contains(httpPortProjects, m.ProjectALL) || funk.Contains(httpPortProjects, belongProject)) &&
			(funk.Contains(httpsPortProjects, m.ProjectALL) || funk.Contains(httpsPortProjects, belongProject)) {
			return true, ""
		}
		reason = fmt.Sprintf("role port belong project %v, not match http %v, https %v", belongProject, httpPortProjects, httpsPortProjects)
		return false, reason
	}

	projects := ctl.GetOwnProjectsFromAlb(alb.Alb)
	if funk.Contains(projects, m.ProjectALL) {
		return true, ""
	}
	if funk.Contains(projects, belongProject) {
		return true, ""
	}
	reason = fmt.Sprintf("role instance, alb project %v ingress project %v", projects, belongProject)
	return false, reason
}

// is our ingressclass
// 1. is our ingressclass
// 2. does not has ingressclass
func (s IngressSelect) CheckShouldHandleViaIngressClass(ing *networkingv1.Ingress) bool {
	ingressclass := GetIngressClass(ing)
	if ingressclass == nil {
		return true
	}
	return *ingressclass == s.cfg.Name
}

func (s IngressSelect) GetIngressBelongProject(obj metav1.Object) string {
	if ns := obj.GetNamespace(); ns != "" {
		nsCr, err := s.drv.Informers.K8s.Namespace.Lister().Get(ns)
		if err != nil {
			s.drv.Log.Error(err, "get namespace failed")
			return ""
		}
		domain := s.cfg.Domain
		if project := nsCr.Labels[fmt.Sprintf("%s/project", domain)]; project != "" {
			return project
		}
	}
	return ""
}
