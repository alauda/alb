package ingress

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"alauda.io/alb2/config"
	ctl "alauda.io/alb2/controller"
	m "alauda.io/alb2/modules"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	"alauda.io/alb2/utils"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/thoas/go-funk"
	"k8s.io/apimachinery/pkg/api/errors"
)

// this is the reconcile
func (c *Controller) Reconcile(key client.ObjectKey) (err error) {
	s := time.Now()
	defer func() {
		c.log.Info("sync ingress over", "key", key, "elapsed", time.Since(s), "err", err)
	}()

	c.log.Info("sync ingress", "key", key)
	alb, err := c.kd.LoadALB(config.GetAlbKey(c))
	if err != nil {
		return err
	}

	ingress, err := c.kd.FindIngress(key)
	if err != nil {
		if errors.IsNotFound(err) {
			c.log.Info("ingress been deleted, cleanup", "key", key)
			return c.cleanUpThisIngress(alb, key)
		}
		c.log.Error(err, "Handle failed", "key", key)
		return err
	}
	c.log.Info("process ingress", "name", ingress.Name, "ns", ingress.Namespace, "ver", ingress.ResourceVersion)
	should, reason := c.shouldHandleIngress(alb, ingress)
	if !should {
		c.log.Info("should not handle this ingress. clean up", "key", key, "reason", reason)
		return c.cleanUpThisIngress(alb, key)
	}

	expect, err := c.generateExpect(alb, ingress)
	if err != nil {
		return err
	}
	_, err = c.doUpdate(ingress, alb, expect, false)

	if err != nil {
		return err
	}
	c.recorder.Event(ingress, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) doUpdate(ing *networkingv1.Ingress, alb *m.AlaudaLoadBalancer, expect *ExpectFtAndRule, dryRun bool) (needdo bool, err error) {
	log := c.log.WithName("do-update").WithValues("ingress", ing.Name, "dry-run", dryRun)
	ohttpft := c.getIngresHttpFtRaw(alb)
	if ohttpft != nil {
		log.Info("ohttp", "ver", ohttpft.ResourceVersion, "svc", ohttpft.Spec.ServiceGroup, "src", ohttpft.Spec.Source)
	}
	httpfa, err := c.genSyncFtAction(ohttpft, expect.http, c.log)
	if err != nil {
		return false, err
	}
	ohttpsft := c.getIngresHttpsFtRaw(alb)
	if ohttpsft != nil {
		log.Info("ohttps", "ver", ohttpsft.ResourceVersion, "https", ohttpsft.Spec.ServiceGroup)
	}
	httpsfa, err := c.genSyncFtAction(ohttpsft, expect.https, c.log)
	if err != nil {
		return false, err
	}
	if dryRun && (httpfa.needDo() || httpsfa.needDo()) {
		return true, nil
	}
	needupdateHttp := httpfa.needDo()
	log.Info("need update http", "updatehttpft", needupdateHttp, "http", httpfa)

	if needupdateHttp {
		if err := c.doUpdateFt(httpfa); err != nil {
			return false, err
		}
		return false, fmt.Errorf("http ft not update,we must sync it first, requeue this ingress")
	}

	needupdateHttps := httpsfa.needDo()
	log.Info("need update https", "updatehttpsft", needupdateHttps, "https", httpsfa)
	if needupdateHttps {
		if err := c.doUpdateFt(httpsfa); err != nil {
			return false, err
		}
		return false, fmt.Errorf("https ft not update,we must create it first, requeue this ingress")
	}

	ohttpRules := c.getIngresHttpFt(alb).FindIngressRuleRaw(IngKey(ing))
	ohttpsRules := c.getIngresHttpsFt(alb).FindIngressRuleRaw(IngKey(ing))

	c.log.Info("do update", "oft-len", len(alb.Frontends), "ohttp-rule-len", len(ohttpsRules), "ehttp-rule-len", len(expect.httpRule), "ohttps-rule", len(ohttpsRules), "ehttps-rule", len(expect.httpsRule))

	httprule, err := c.genSyncRuleAction("http", ing, ohttpRules, expect.httpRule, c.log)
	if err != nil {
		return false, err
	}
	httpsrule, err := c.genSyncRuleAction("https", ing, ohttpsRules, expect.httpsRule, c.log)
	if err != nil {
		return false, err
	}

	if dryRun && (httprule.needDo() || httpsrule.needDo()) {
		return true, nil
	}

	if err := c.doUpdateRule(httprule); err != nil {
		return false, err
	}
	if err := c.doUpdateRule(httpsrule); err != nil {
		return false, err

	}
	return false, nil
}

type SyncFt struct {
	Create []*alb2v1.Frontend
	Update []*alb2v1.Frontend
	Delete []*alb2v1.Frontend
}

func (f *SyncFt) needDo() bool {
	if f == nil {
		return false
	}
	return len(f.Create) != 0 || len(f.Delete) != 0 || len(f.Update) != 0
}

func (c *Controller) genSyncFtAction(oft *alb2v1.Frontend, eft *alb2v1.Frontend, log logr.Logger) (*SyncFt, error) {
	if oft == nil && eft == nil {
		return nil, nil
	}
	if oft == nil && eft != nil {
		if eft.ResourceVersion != "" {
			log.Info("eft already exist?", "name", eft.Name, "ver", eft.ResourceVersion)
		}
		log.Info("we want to create ft", "name", eft.Name, "source", eft.Spec.Source, "ver", eft.ResourceVersion)
		return &SyncFt{
			Create: []*alb2v1.Frontend{eft},
		}, nil
	}
	if oft != nil && eft == nil {
		log.Info("expect to delete ft? it may happens when change to https only mode.but just ignore it now")
		return nil, nil
	}
	// we only update ft when source group change to exist
	if oft != nil && eft != nil {
		log.Info("ft both exist", "oft-v", oft.ResourceVersion, "oft-src", oft.Spec.Source, "oft-svc", oft.Spec.ServiceGroup, "eft-src", eft.Spec.Source, "eft-svc", eft.Spec.ServiceGroup)
		// oft it get from k8s, eft is crate via our. we should update the exist ft.
		if oft.Spec.ServiceGroup == nil && eft.Spec.ServiceGroup != nil {
			log.Info("ft default backend changed ", "backend", eft.Spec.ServiceGroup, "source", eft.Spec.Source)
			f := oft.DeepCopy()
			f.Spec = eft.Spec
			return &SyncFt{
				Update: []*alb2v1.Frontend{f},
			}, nil
		}
	}
	return nil, nil
}

func (c *Controller) doUpdateFt(fa *SyncFt) error {
	c.log.Info("do update ft", "crate", len(fa.Create), "update", len(fa.Update), "delete", len(fa.Delete))
	if !fa.needDo() {
		return nil
	}
	for _, f := range fa.Create {
		cft, err := c.kd.CreateFt(f)
		if err != nil {
			return err
		}
		c.log.Info("ft not-synced create it", "eft", f.Name, "ver", f.ResourceVersion, "id", f.UID, "cid", cft.UID)
	}
	for _, f := range fa.Update {
		c.log.Info("ft not-synced update it ", "sepc", f.Spec)
		cft, err := c.kd.UpdateFt(f)
		if err != nil {
			return err
		}
		c.log.Info("ft not-synced update it", "eft", f.Name, "ver", f.ResourceVersion, "id", f.UID, "cid", cft.UID)
	}
	return nil
}

type ExpectFtAndRule struct {
	http      *alb2v1.Frontend
	httpRule  []*alb2v1.Rule
	https     *alb2v1.Frontend
	httpsRule []*alb2v1.Rule
}

func (e *ExpectFtAndRule) ShowExpect() string {
	msg := ""
	if e.http != nil {
		msg += fmt.Sprintf("httpft: %+v", e.http)
		msg += fmt.Sprintf("httpft-len: %v", len(e.httpRule))
		for _, r := range e.httpRule {
			msg += fmt.Sprintf("rule : %+v", r)
		}
	}
	if e.https != nil {
		msg += fmt.Sprintf("httpsft: %+v", e.https)
		msg += fmt.Sprintf("httpsft-len: %v", len(e.httpsRule))
		for _, r := range e.httpsRule {
			msg += fmt.Sprintf("rule : %+v", r)
		}
	}
	return msg
}

// generate expect ft and rule fot this ingress
func (c *Controller) generateExpect(alb *m.AlaudaLoadBalancer, ingress *networkingv1.Ingress) (*ExpectFtAndRule, error) {
	httpFt, httpsFt, err := c.generateExpectFrontend(alb, ingress)
	if err != nil {
		return nil, err
	}

	httpRule := []*alb2v1.Rule{}
	httpsRule := []*alb2v1.Rule{}

	// generate epxpect rules
	for rIndex, r := range ingress.Spec.Rules {
		host := strings.ToLower(r.Host)
		if r.HTTP == nil {
			c.log.Info("a empty ingress", "ns", ingress.Namespace, "name", ingress.Name, "host", host)
			continue
		}
		for pIndex, p := range r.HTTP.Paths {
			if httpFt != nil {
				rule, err := c.generateRule(ingress, alb.GetAlbKey(), httpFt, host, p, rIndex, pIndex)
				if err != nil {
					return nil, err
				}
				httpRule = append(httpRule, rule)
			}
			if httpsFt != nil {
				rule, err := c.generateRule(ingress, alb.GetAlbKey(), httpsFt, host, p, rIndex, pIndex)
				if err != nil {
					return nil, err
				}
				httpsRule = append(httpsRule, rule)
			}
		}
	}

	return &ExpectFtAndRule{
		http:      httpFt,
		https:     httpsFt,
		httpRule:  httpRule,
		httpsRule: httpsRule,
	}, nil
}

func (c *Controller) cleanUpThisIngress(alb *m.AlaudaLoadBalancer, key client.ObjectKey) error {
	IngressHTTPPort := config.GetInt("INGRESS_HTTP_PORT")
	IngressHTTPSPort := config.GetInt("INGRESS_HTTPS_PORT")
	log := c.log.WithName("cleanup").WithValues("ingress", key)
	log.Info("clean up")
	var ft *m.Frontend
	for _, f := range alb.Frontends {
		if int(f.Spec.Port) == IngressHTTPPort || int(f.Spec.Port) == IngressHTTPSPort {
			ft = f
			// 如果这个ft是因为这个ingress创建起来的,当ingress被删除时,要更新这个ft
			createBythis := ft.IsCreateByThisIngress(key.Namespace, key.Name)
			log.Info("find ft", "createBythis", createBythis, "backend", ft.Spec.ServiceGroup, "souce", ft.Spec.Source)
			if createBythis {
				// wipe default backend
				ft.Spec.ServiceGroup = nil
				ft.Spec.Source = nil
				ft.Spec.BackendProtocol = ""
				log.Info("wipe ft default backend cause of ingres been delete", "nft-ver", ft.ResourceVersion, "ft", ft.Name, "ingress", key)
				nft, err := c.kd.UpdateFt(ft.Frontend)
				if err != nil {
					log.Error(nil, fmt.Sprintf("upsert ft failed: %s", err))
					return err
				}
				log.Info("after update", "nft-ver", nft.ResourceVersion, "svc", nft.Spec.ServiceGroup)
			}

			for _, rule := range ft.Rules {
				if rule.IsCreateByThisIngress(key.Namespace, key.Name) {
					log.Info(fmt.Sprintf("delete-rules  ingress key: %s  rule name %s reason: ingress-delete", key, rule.Name))
					err := c.kd.DeleteRule(rule.Key())
					if err != nil && !errors.IsNotFound(err) {
						log.Error(err, "delete rule fial", "rule", rule.Name)
						return err
					}
				}
			}
		}
	}
	// TODO we should find all should handled ingress ,and find one which has defaultBackend.now just left this job to resync.
	return nil
}

func (c *Controller) shouldHandleIngress(alb *m.AlaudaLoadBalancer, ing *networkingv1.Ingress) (rv bool, reason string) {
	IngressHTTPPort := c.GetIngressHttpPort()
	IngressHTTPSPort := c.GetIngressHttpsPort()
	_, err := c.GetIngressClass(ing, c.icConfig)
	if err != nil {
		reason = fmt.Sprintf("Ignoring ingress because of error while validating ingress class %v %v", ing, err)
		return false, reason
	}

	belongProject := c.GetIngressBelongProject(ing)
	role := ctl.GetAlbRoleType(alb.Labels)
	if role == ctl.RolePort {
		hasHTTPPort := false
		hasHTTPSPort := false
		httpPortProjects := []string{}
		httpsPortProjects := []string{}
		for _, ft := range alb.Frontends {
			if ft.SamePort(IngressHTTPPort) {
				hasHTTPPort = true
				httpPortProjects = ctl.GetOwnProjects(ft.Name, ft.Labels)
			} else if ft.SamePort(IngressHTTPSPort) {
				hasHTTPSPort = true
				httpsPortProjects = ctl.GetOwnProjects(ft.Name, ft.Labels)
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

	projects := ctl.GetOwnProjects(alb.Name, alb.Labels)
	if funk.Contains(projects, m.ProjectALL) {
		return true, ""
	}
	if funk.Contains(projects, belongProject) {
		return true, ""
	}
	reason = fmt.Sprintf("role instance,project %v belog %v", projects, belongProject)
	return false, reason
}

func (c *Controller) generateExpectFrontend(alb *m.AlaudaLoadBalancer, ingress *networkingv1.Ingress) (http *alb2v1.Frontend, https *alb2v1.Frontend, err error) {

	IngressHTTPPort := c.GetIngressHttpPort()
	IngressHTTPSPort := c.GetIngressHttpsPort()
	defaultSSLCert := strings.ReplaceAll(c.GetDefaultSSLSCert(), "/", "_")

	alblabelKey := fmt.Sprintf(config.Get("labels.name"), config.Get("DOMAIN"))
	need := getIngressFtTypes(ingress, c)

	var albhttpFt *alb2v1.Frontend
	var albhttpsFt *alb2v1.Frontend

	if need.NeedHttp() && c.getIngresHttpFt(alb) != nil {
		// NOTE: do not change alb.
		albhttpFt = c.getIngresHttpFt(alb).DeepCopy()
	}
	if need.NeedHttps() && c.getIngresHttpsFt(alb) != nil {
		// NOTE: do not change alb.
		albhttpsFt = c.getIngresHttpsFt(alb).DeepCopy()
	}

	if need.NeedHttp() && albhttpFt == nil {
		name := fmt.Sprintf("%s-%05d", alb.Name, IngressHTTPPort)
		c.log.Info("need http ft and ft not exist create one", "name", name)

		albhttpFt = &alb2v1.Frontend{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: alb.Namespace,
				Labels: map[string]string{
					alblabelKey: alb.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: alb2v1.SchemeGroupVersion.String(),
						Kind:       alb2v1.ALB2Kind,
						Name:       alb.Name,
						UID:        alb.UID,
					},
				},
			},
			Spec: alb2v1.FrontendSpec{
				Port:     alb2v1.PortNumber(IngressHTTPPort),
				Protocol: m.ProtoHTTP,
				Source: &alb2v1.Source{
					Name:      ingress.Name,
					Namespace: ingress.Namespace,
					Type:      m.TypeIngress,
				},
			},
		}
	}
	if need.NeedHttps() && albhttpsFt == nil {
		name := fmt.Sprintf("%s-%05d", alb.Name, IngressHTTPSPort)
		c.log.Info("need https ft and ft not exist create one", "name", name)
		albhttpsFt = &alb2v1.Frontend{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: alb.Namespace,
				Name:      name,
				Labels: map[string]string{
					alblabelKey: alb.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: alb2v1.SchemeGroupVersion.String(),
						Kind:       alb2v1.ALB2Kind,
						Name:       alb.Name,
						UID:        alb.UID,
					},
				},
			},
			Spec: alb2v1.FrontendSpec{
				Port:            alb2v1.PortNumber(IngressHTTPSPort),
				Protocol:        m.ProtoHTTPS,
				CertificateName: defaultSSLCert,
				Source: &alb2v1.Source{
					Name:      ingress.Name,
					Namespace: ingress.Namespace,
					Type:      m.TypeIngress,
				},
			},
		}
	}

	// for default backend, we will not create rules but save services to frontends' service-group
	if HasDefaultBackend(ingress) {
		c.log.Info("ingress has default backend", "ing", ingress.Name)
		// just update it, let the resolver decide should we really upate it.
		annotations := ingress.GetAnnotations()
		backendProtocol := strings.ToLower(annotations[ALBBackendProtocolAnnotation])
		defaultBackendService := ingress.Spec.DefaultBackend.Service
		portInService, err := c.kd.GetServicePortNumber(ingress.Namespace, defaultBackendService.Name, ToInStr(defaultBackendService.Port), corev1.ProtocolTCP)
		svc := &alb2v1.ServiceGroup{
			Services: []alb2v1.Service{
				{
					Namespace: ingress.Namespace,
					Name:      defaultBackendService.Name,
					Port:      portInService,
					Weight:    100,
				},
			},
		}
		if err != nil {
			return nil, nil, err
		}
		if albhttpFt != nil {
			m.SetDefaultBackend(albhttpFt, backendProtocol, svc)
			m.SetSource(albhttpFt, ingress)
		}
		if albhttpsFt != nil {
			m.SetDefaultBackend(albhttpsFt, backendProtocol, svc)
			m.SetSource(albhttpsFt, ingress)
		}
	}
	return albhttpFt, albhttpsFt, nil
}

func (c *Controller) getIngresHttpFt(alb *m.AlaudaLoadBalancer) *m.Frontend {
	http := c.GetIngressHttpPort()
	return alb.FindIngressFt(http, m.ProtoHTTP)
}

func (c *Controller) getIngresHttpsFt(alb *m.AlaudaLoadBalancer) *m.Frontend {
	https := c.GetIngressHttpsPort()
	return alb.FindIngressFt(https, m.ProtoHTTPS)
}

func (c *Controller) getIngresHttpFtRaw(alb *m.AlaudaLoadBalancer) *alb2v1.Frontend {
	http := c.GetIngressHttpPort()
	return alb.FindIngressFtRaw(http, m.ProtoHTTP)
}

func (c *Controller) getIngresHttpsFtRaw(alb *m.AlaudaLoadBalancer) *alb2v1.Frontend {
	https := c.GetIngressHttpsPort()
	return alb.FindIngressFtRaw(https, m.ProtoHTTPS)
}

func (c *Controller) generateRule(
	ingress *networkingv1.Ingress,
	albKey client.ObjectKey,
	ft *alb2v1.Frontend,
	host string,
	ingresPath networkingv1.HTTPIngressPath,
	ruleIndex int,
	pathIndex int,
) (*alb2v1.Rule, error) {
	ALBSSLAnnotation := fmt.Sprintf("alb.networking.%s/tls", c.GetDomain())

	ingInfo := fmt.Sprintf("%s/%s:%v:%v", ingress.Namespace, ingress.Name, ft.Spec.Port, pathIndex)

	annotations := ingress.GetAnnotations()
	rewriteTarget := annotations[ALBRewriteTargetAnnotation]
	vhost := annotations[ALBVHostAnnotation]
	enableCORS := annotations[ALBEnableCORSAnnotation] == "true"
	corsAllowHeaders := annotations[ALBCORSAllowHeadersAnnotation]
	corsAllowOrigin := annotations[ALBCORSAllowOriginAnnotation]
	backendProtocol := strings.ToLower(annotations[ALBBackendProtocolAnnotation])
	var (
		redirectURL  string
		redirectCode int
	)
	if annotations[ALBPermanentRedirectAnnotation] != "" && annotations[ALBTemporalRedirectAnnotation] != "" {
		c.log.Error(nil, fmt.Sprintf("cannot use PermanentRedirect and TemporalRedirect at same time, ingress %s", ingInfo))
		return nil, nil
	}
	if annotations[ALBPermanentRedirectAnnotation] != "" {
		redirectURL = annotations[ALBPermanentRedirectAnnotation]
		redirectCode = 301
	}
	if annotations[ALBTemporalRedirectAnnotation] != "" {
		redirectURL = annotations[ALBTemporalRedirectAnnotation]
		redirectCode = 302
	}

	ruleAnnotation := ctl.GenerateRuleAnnotationFromIngressAnnotation(ingress.Name, annotations)

	certs := make(map[string]string)

	if backendProtocol != "" && !ValidBackendProtocol[backendProtocol] {
		c.log.Error(nil, fmt.Sprintf("Unsupported backend protocol %s for ingress %s", backendProtocol, ingInfo))
		return nil, nil
	}

	for _, tls := range ingress.Spec.TLS {
		for _, host := range tls.Hosts {
			// NOTE: 与前端约定保密字典用_，全局证书用/分割
			certs[strings.ToLower(host)] = fmt.Sprintf("%s_%s", ingress.GetNamespace(), tls.SecretName)
		}
	}
	sslMap := parseSSLAnnotation(annotations[ALBSSLAnnotation])
	for host, cert := range sslMap {
		// should not override spec.tls
		if certs[strings.ToLower(host)] == "" {
			certs[strings.ToLower(host)] = cert
		}
	}

	ingressBackend := ingresPath.Backend.Service
	portInService, err := c.kd.GetServicePortNumber(ingress.Namespace, ingressBackend.Name, ToInStr(ingressBackend.Port), corev1.ProtocolTCP)
	if err != nil {
		return nil, fmt.Errorf("get port in svc %s/%s %v fail err %v", ingress.Namespace, ingressBackend.Name, ingressBackend.Port, err)
	}
	url := ingresPath.Path
	pathType := networkingv1.PathTypeImplementationSpecific
	if ingresPath.PathType != nil {
		pathType = *ingresPath.PathType
	}
	sourceIngressVersion := c.GetLabelSourceIngressVer()
	ruleAnnotation[sourceIngressVersion] = ingress.ResourceVersion
	ruleAnnotation[c.GetLabelSourceIngressPathIndex()] = fmt.Sprintf("%d", pathIndex)
	ruleAnnotation[c.GetLabelSourceIngressRuleIndex()] = fmt.Sprintf("%d", ruleIndex)
	name := strings.ToLower(utils.RandomStr(ft.Name, 4))
	ruleSpec := alb2v1.RuleSpec{
		Domain:           host,
		URL:              url,
		DSLX:             GetDSLX(host, url, pathType),
		Priority:         DefaultPriority, // TODO ability to set priority in ingress.
		RewriteBase:      url,
		RewriteTarget:    rewriteTarget,
		BackendProtocol:  backendProtocol,
		CertificateName:  certs[host],
		EnableCORS:       enableCORS,
		CORSAllowHeaders: corsAllowHeaders,
		CORSAllowOrigin:  corsAllowOrigin,
		RedirectURL:      redirectURL,
		RedirectCode:     redirectCode,
		VHost:            vhost,
		Description:      ingInfo,
		ServiceGroup: &alb2v1.ServiceGroup{ //TODO ingress could only have one service as backend
			Services: []alb2v1.Service{
				{
					Namespace: ingress.Namespace,
					Name:      ingresPath.Backend.Service.Name,
					Port:      portInService,
					Weight:    100,
				},
			},
		},
		Source: &alb2v1.Source{
			Type:      m.TypeIngress,
			Namespace: ingress.Namespace,
			Name:      ingress.Name,
		},
	}
	if err != nil {
		c.log.Error(err, "")
		return nil, err
	}

	ruleRes := &alb2v1.Rule{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ft.Namespace,
			Annotations: ruleAnnotation,
			Labels: map[string]string{
				fmt.Sprintf(config.Get("labels.name"), config.Get("DOMAIN")):     albKey.Name,
				fmt.Sprintf(config.Get("labels.frontend"), config.Get("DOMAIN")): ft.Name,
				config.GetLabelSourceType():                                      m.TypeIngress,
				config.GetLabelSourceIngressHash():                               hashSource(ruleSpec.Source),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: alb2v1.SchemeGroupVersion.String(),
					Kind:       alb2v1.FrontendKind,
					Name:       ft.Name,
					UID:        ft.UID,
				},
			},
		},
		Spec: ruleSpec,
	}
	return ruleRes, nil
}

type SyncRule struct {
	Create []*alb2v1.Rule
	Delete []*alb2v1.Rule
}

func (r *SyncRule) needDo() bool {
	if r == nil {
		return false
	}
	return len(r.Create) != 0 || len(r.Delete) != 0
}

func (c *Controller) doUpdateRule(r *SyncRule) error {
	if !r.needDo() {
		return nil
	}

	// we want create rule first. since that if this pod crash, it will not lossing rule at least.
	for _, r := range r.Create {
		crule, err := c.kd.CreateRule(r)
		if err != nil {
			c.log.Error(err, "create rule fail", "rule", r.Name)
			return err
		}
		c.log.Info("create rule ok", "rule", crule.Name, "id", crule.UID)
	}
	for _, r := range r.Delete {
		if err := c.kd.DeleteRule(m.RuleKey(r)); err != nil && errors.IsNotFound(err) {
			c.log.Error(err, "delete rule fail", "rule", r.Name)
			return err
		}
		c.log.Info("delete rule ok", "rule", r.Name)
	}
	return nil
}

func (c *Controller) genSyncRuleAction(kind string, ing *networkingv1.Ingress, existRules []*alb2v1.Rule, expectRules []*alb2v1.Rule, log logr.Logger) (*SyncRule, error) {
	log = log.WithValues("kind", kind, "ing", ing.Name, "ing-ver", ing.ResourceVersion)
	log.Info("do update rule", "exist-rule-len", len(existRules), "expect-rule-len", len(expectRules))

	showRule := func(r *alb2v1.Rule) []interface{} {
		indexDesc := c.ShowRuleIndex(r)
		pathDesc := ""
		{
			p, err := c.AccessIngressPathViaRule(ing, r)
			if err != nil {
				pathDesc = fmt.Sprintf("%v", err)
			} else {
				pathDesc = p.Path
			}
		}
		tags := []interface{}{"name", r.Name, "hash", ruleHash(r), "ingress", r.Spec.Source.Name, "ing-ver", c.ruleIngressVer(r), "index", indexDesc, "path", pathDesc}
		if c.DebugRuleSync() {
			tags = append(tags, "id", ruleIdentify(r))
		}
		return tags
	}
	_ = showRule

	// need delete
	needDelete := []*alb2v1.Rule{}
	needCreate := []*alb2v1.Rule{}
	existRuleHash := map[string]bool{}
	expectRuleHash := map[string]bool{}
	// we need a layerdmap  ingress/ver/rule-indx/path-index/exist|expect/hash: v
	for _, r := range expectRules {
		hash := ruleHash(r)
		log.Info("expect rule ", "name", r.Name, "hash", hash)
		expectRuleHash[hash] = true
	}
	for _, r := range existRules {
		hash := ruleHash(r)
		log.Info("exist rule ", "name", r.Name, "hash", hash)
		existRuleHash[hash] = true
	}

	for _, r := range existRules {
		hash := ruleHash(r)
		if !expectRuleHash[hash] {
			needDelete = append(needDelete, r)
		}
	}
	for _, r := range expectRules {
		hash := ruleHash(r)
		if !existRuleHash[hash] {
			needCreate = append(needCreate, r)
		}
	}
	if len(needCreate) == 0 && len(needDelete) == 0 {
		log.Info("nothing need todo ingress synced.")
	}

	for _, r := range needCreate {
		log.Info("should create rule", showRule(r)...)
	}
	for _, r := range needDelete {
		log.Info("should delete rule", showRule(r)...)
	}

	// we want create rule first. since that if this popd crash, it will not lossing rule at least.
	return &SyncRule{
		Create: needCreate,
		Delete: needDelete,
	}, nil
}

// rule which have same identity considered as same rule
// we may add some label/annotation in rule,such as creator/update time, etc
func ruleIdentify(r *alb2v1.Rule) string {
	label := []string{}
	annotation := []string{}
	spec := r.Spec
	// do care about the other hash
	for k, v := range r.Labels {
		if strings.HasPrefix(k, "alb") {
			label = append(label, k+":"+v)
		}
	}
	for k, v := range r.Annotations {
		if strings.HasPrefix(k, "alb") {
			annotation = append(annotation, k+":"+v)
		}
	}
	// map are unordered.
	sort.Strings(label)
	sort.Strings(annotation)
	var b bytes.Buffer

	for _, v := range label {
		b.WriteString(v)
	}
	for _, v := range annotation {
		b.WriteString(v)
	}
	b.WriteString(spec.Identity())
	for _, owner := range r.OwnerReferences {
		b.WriteString(string(owner.UID))
	}
	return b.String()
}

func ruleHash(r *alb2v1.Rule) string {
	id := ruleIdentify(r)
	h := sha256.New()
	h.Write([]byte(id))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Controller) ruleIngressVer(r *alb2v1.Rule) string {
	sourceIngressVersion := c.GetLabelSourceIngressVer()
	return r.Annotations[sourceIngressVersion]
}

func (c *Controller) ruleIngressPathIndex(r *alb2v1.Rule) string {
	return r.Annotations[c.GetLabelSourceIngressPathIndex()]
}

func (c *Controller) ruleIngressRuleIndex(r *alb2v1.Rule) string {
	return r.Annotations[c.GetLabelSourceIngressPathIndex()]
}

func GetDSLX(domain, url string, pathType networkingv1.PathType) alb2v1.DSLX {
	var dslx alb2v1.DSLX
	if url != "" {
		if pathType == networkingv1.PathTypeExact {
			dslx = append(dslx, alb2v1.DSLXTerm{
				Values: [][]string{
					{utils.OP_EQ, url},
				},
				Type: utils.KEY_URL,
			})
		} else if pathType == networkingv1.PathTypePrefix {
			dslx = append(dslx, alb2v1.DSLXTerm{
				Values: [][]string{
					{utils.OP_STARTS_WITH, url},
				},
				Type: utils.KEY_URL,
			})
		} else {
			// path is regex
			if strings.ContainsAny(url, "^$():?[]*\\") {
				if !strings.HasPrefix(url, "^") {
					url = "^" + url
				}

				dslx = append(dslx, alb2v1.DSLXTerm{
					Values: [][]string{
						{utils.OP_REGEX, url},
					},
					Type: utils.KEY_URL,
				})
			} else {
				dslx = append(dslx, alb2v1.DSLXTerm{
					Values: [][]string{
						{utils.OP_STARTS_WITH, url},
					},
					Type: utils.KEY_URL,
				})
			}
		}
	}

	if domain != "" {
		if strings.HasPrefix(domain, "*") {
			dslx = append(dslx, alb2v1.DSLXTerm{
				Values: [][]string{
					{utils.OP_ENDS_WITH, domain},
				},
				Type: utils.KEY_HOST,
			})
		} else {
			dslx = append(dslx, alb2v1.DSLXTerm{
				Values: [][]string{
					{utils.OP_EQ, domain},
				},
				Type: utils.KEY_HOST,
			})
		}
	}
	return dslx
}

func (c *Controller) AccessIngressPathViaRule(ing *networkingv1.Ingress, rule *alb2v1.Rule) (*networkingv1.HTTPIngressPath, error) {
	rIndexStr := rule.Annotations[c.GetLabelSourceIngressRuleIndex()]
	rIndex, err := strconv.Atoi(rIndexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid rule index %v", rIndexStr)
	}
	pIndexStr := rule.Annotations[c.GetLabelSourceIngressPathIndex()]
	pIndex, err := strconv.Atoi(pIndexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid path index %v", pIndexStr)
	}

	if len(ing.Spec.Rules) <= rIndex {
		return nil, fmt.Errorf("%v out of range of rule %v", rIndex, len(ing.Spec.Rules))
	}
	r := ing.Spec.Rules[rIndex]
	if r.HTTP == nil {
		return nil, fmt.Errorf("http in nill")
	}
	if len(r.HTTP.Paths) <= pIndex {
		return nil, fmt.Errorf("%v out of range of path %v", pIndex, len(r.HTTP.Paths))
	}
	p := r.HTTP.Paths[pIndex]
	return &p, nil
}

func (c *Controller) ShowRuleIndex(rule *alb2v1.Rule) string {
	return fmt.Sprintf("%s:%s", rule.Annotations[c.GetLabelSourceIngressRuleIndex()], rule.Annotations[c.GetLabelSourceIngressPathIndex()])
}
