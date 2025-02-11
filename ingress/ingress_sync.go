package ingress

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"alauda.io/alb2/config"
	m "alauda.io/alb2/controller/modules"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	alb2v2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	"alauda.io/alb2/utils"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
)

// this is the reconcile
func (c *Controller) Reconcile(key client.ObjectKey) (requeue bool, err error) {
	s := time.Now()
	log := c.log.WithValues("key", key)
	defer func() {
		log.Info("sync ingress over", "elapsed", time.Since(s), "err", err)
	}()

	log.Info("reconcile ingress")
	alb, err := c.kd.LoadALB(config.GetAlbKey(c.Config))
	if err != nil {
		return false, err
	}

	ingress, err := c.kd.FindIngress(key)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("ingress been deleted, cleanup")
			return false, c.cleanUpThisIngress(alb, key)
		}
		log.Error(err, "Handle failed")
		return false, err
	}
	log = log.WithValues("ver", ingress.ResourceVersion)
	should, reason := c.ShouldHandleIngress(alb, ingress)
	if !should {
		log.Info("should not handle this ingress. clean up", "reason", reason)
		return false, c.cleanUpThisIngress(alb, key)
	}
	project := c.GetIngressBelongProject(ingress)
	expect, err := c.generateExpect(alb, ingress, project)
	if err != nil {
		return false, err
	}
	_, err = c.doUpdate(ingress, alb, expect, false)
	if err != nil {
		return false, err
	}
	c.recorder.Event(ingress, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)

	if c.NeedUpdateIngressStatus(alb, ingress) {
		log.Info("need update ingress status")
		return false, c.UpdateIngressStatus(alb, ingress)
	} else {
		log.Info("not need update ingress status")
	}
	return false, nil
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
	log.Info("need update http ft?", "need", needupdateHttp, "http", utils.PrettyJson(httpfa))

	if needupdateHttp {
		if err := c.doUpdateFt(httpfa); err != nil {
			return false, err
		}
		return false, fmt.Errorf("http ft not update,we must sync it first, requeue this ingress")
	}

	needupdateHttps := httpsfa.needDo()
	log.Info("need update https ft?", "need", needupdateHttps, "https", httpsfa)
	if needupdateHttps {
		if err := c.doUpdateFt(httpsfa); err != nil {
			return false, err
		}
		return false, fmt.Errorf("https ft not update,we must create it first, requeue this ingress")
	}

	ohttpRules := c.getIngresHttpFt(alb).FindIngressRuleRaw(IngKey(ing))
	ohttpsRules := c.getIngresHttpsFt(alb).FindIngressRuleRaw(IngKey(ing))

	c.log.Info("check update", "oft-len", len(alb.Frontends), "ohttp-rule-len", len(ohttpsRules), "ehttp-rule-len", len(expect.httpRule), "ohttps-rule", len(ohttpsRules), "ehttps-rule", len(expect.httpsRule))

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
	// user may update those ingress created ft
	if oft != nil && eft != nil {
		log.Info("ft both exist", "oft-v", oft.ResourceVersion, "oft-src", oft.Spec.Source, "oft-svc", oft.Spec.ServiceGroup, "eft-src", eft.Spec.Source, "eft-svc", eft.Spec.ServiceGroup)
		// oft it get from k8s, eft is crate via our. we should update the exist ft.
		if oft.Spec.ServiceGroup == nil && eft.Spec.ServiceGroup != nil {
			log.Info("ft default backend changed ", "backend", eft.Spec.ServiceGroup, "source", eft.Spec.Source)
			f := oft.DeepCopy()
			f.Spec.ServiceGroup = eft.Spec.ServiceGroup
			f.Spec.Source = eft.Spec.Source
			return &SyncFt{
				Update: []*alb2v1.Frontend{f},
			}, nil
		}
	}
	return nil, nil
}

func (c *Controller) doUpdateFt(fa *SyncFt) error {
	c.log.Info("check update ft", "crate", len(fa.Create), "update", len(fa.Update), "delete", len(fa.Delete))
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
		c.log.Info("ft not-synced update it ", "spec", f.Spec)
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
func (c *Controller) generateExpect(alb *m.AlaudaLoadBalancer, ingress *networkingv1.Ingress, project string) (*ExpectFtAndRule, error) {
	httpFt, httpsFt, err := c.generateExpectFrontend(alb, ingress)
	if err != nil {
		return nil, err
	}

	httpRule := []*alb2v1.Rule{}
	httpsRule := []*alb2v1.Rule{}

	// generate expect rules
	for rIndex, r := range ingress.Spec.Rules {
		host := strings.ToLower(r.Host)
		if r.HTTP == nil {
			c.log.Info("a empty ingress", "ns", ingress.Namespace, "name", ingress.Name, "host", host)
			continue
		}
		for pIndex, p := range r.HTTP.Paths {
			if httpFt != nil {
				rule, err := c.GenerateRule(ingress, alb.GetAlbKey(), httpFt, rIndex, pIndex, project)
				if err != nil {
					c.log.Error(err, "generate http rule fail", "ingress", ingress.Name, "ns", ingress.Namespace, "path", p.String(), "rindex", rIndex, "pindex", pIndex)
					continue
				}
				httpRule = append(httpRule, rule)
			}
			if httpsFt != nil {
				rule, err := c.GenerateRule(ingress, alb.GetAlbKey(), httpsFt, rIndex, pIndex, project)
				if err != nil {
					c.log.Error(err, "generate https rule fail", "ingress", ingress.Name, "ns", ingress.Namespace, "path", p.String(), "rindex", rIndex, "pindex", pIndex)
					continue
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
	IngressHTTPPort := c.GetIngressHttpPort()
	IngressHTTPSPort := c.GetIngressHttpsPort()
	log := c.log.WithName("cleanup").WithValues("ingress", key)
	log.Info("clean up")
	var ft *m.Frontend
	for _, f := range alb.Frontends {
		if int(f.Spec.Port) == IngressHTTPPort || int(f.Spec.Port) == IngressHTTPSPort {
			ft = f
			// 如果这个 ft 是因为这个 ingress 创建起来的，当 ingress 被删除时，要更新这个 ft
			createBythis := ft.IsCreateByThisIngress(key.Namespace, key.Name)
			log.Info("find ft", "createBythis", createBythis, "backend", ft.Spec.ServiceGroup, "source", ft.Spec.Source)
			if createBythis {
				// wipe default backend
				ft.Spec.ServiceGroup = nil
				ft.Spec.Source = nil
				ft.Spec.BackendProtocol = ""
				log.Info("wipe ft default backend cause of ingres been delete", "nft-ver", ft.ResourceVersion, "ft", ft.Name, "ingress", key)
				nft, err := c.kd.UpdateFt(ft.Frontend)
				if err != nil {
					log.Error(nil, fmt.Sprintf("update ft failed: %s", err))
					return err
				}
				log.Info("after update", "nft-ver", nft.ResourceVersion, "svc", nft.Spec.ServiceGroup)
			}

			for _, rule := range ft.Rules {
				if rule.IsCreateByThisIngress(key.Namespace, key.Name) {
					log.Info(fmt.Sprintf("delete-rules  ingress key: %s  rule name %s reason: ingress-delete", key, rule.Name))
					err := c.kd.DeleteRule(rule.Key())
					if err != nil && !errors.IsNotFound(err) {
						log.Error(err, "delete rule fail", "rule", rule.Name)
						return err
					}
				}
			}
		}
	}
	// TODO we should find all should handled ingress ,and find one which has defaultBackend.now just left this job to resync.
	return nil
}

// return: nil or not-empty string
func GetIngressClass(ing *networkingv1.Ingress) *string {
	var ingressclass *string
	// get ingress class from ing.spec
	if ing.Spec.IngressClassName != nil {
		ingressclass = ing.Spec.IngressClassName
	}
	// try annotation
	if ingressclass == nil {
		if ingressClassFromAnnotation, ok := ing.GetAnnotations()[config.IngressKey]; ok {
			ingressclass = &ingressClassFromAnnotation
		}
	}
	if ingressclass != nil && strings.TrimSpace(*ingressclass) == "" {
		ingressclass = nil
	}
	return ingressclass
}

func (c *Controller) generateExpectFrontend(alb *m.AlaudaLoadBalancer, ingress *networkingv1.Ingress) (http *alb2v1.Frontend, https *alb2v1.Frontend, err error) {
	IngressHTTPPort := c.GetIngressHttpPort()
	IngressHTTPSPort := c.GetIngressHttpsPort()
	defaultSSLCert := strings.ReplaceAll(c.GetDefaultSSLCert(), "/", "_")

	alblabelKey := c.GetLabelAlbName()
	need := getIngressFtTypes(ingress, c.Config)
	c.log.Info("ing need http https", "http", need.NeedHttp(), "https", need.NeedHttps())

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
		name := fmt.Sprintf("%s-%05d", alb.Alb.Name, IngressHTTPPort)
		c.log.Info("need http ft and ft not exist. create one", "name", name)

		albhttpFt = &alb2v1.Frontend{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: alb.Alb.Namespace,
				Labels: map[string]string{
					alblabelKey: alb.Alb.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: alb2v2.SchemeGroupVersion.String(),
						Kind:       alb2v1.ALB2Kind,
						Name:       alb.Alb.Name,
						UID:        alb.Alb.UID,
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
	// TODO 当 alb 的默认证书变化时，已经创建的 ft 的默认证书不会变化，需要用户自己在去更新 ft 上的证书
	if need.NeedHttps() && albhttpsFt == nil {
		name := fmt.Sprintf("%s-%05d", alb.Alb.Name, IngressHTTPSPort)
		c.log.Info("need https ft and ft not exist create one", "name", name)
		albhttpsFt = &alb2v1.Frontend{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: alb.Alb.Namespace,
				Name:      name,
				Labels: map[string]string{
					alblabelKey: alb.Alb.Name,
				},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: alb2v2.SchemeGroupVersion.String(),
						Kind:       alb2v2.ALB2Kind,
						Name:       alb.Alb.Name,
						UID:        alb.Alb.UID,
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
		// just update it, let the resolver decide should we really update it.
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
	if albhttpsFt != nil {
		albhttpsFt.Spec.CertificateName = defaultSSLCert
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
	ft := alb.FindIngressFtRaw(http, m.ProtoHTTP)
	return ft
}

func (c *Controller) getIngresHttpsFtRaw(alb *m.AlaudaLoadBalancer) *alb2v1.Frontend {
	https := c.GetIngressHttpsPort()
	return alb.FindIngressFtRaw(https, m.ProtoHTTPS)
}

func (c *Controller) GenerateRule(
	ingress *networkingv1.Ingress,
	albKey client.ObjectKey,
	ft *alb2v1.Frontend,
	ruleIndex int,
	pathIndex int,
	project string,
) (*alb2v1.Rule, error) {
	r := ingress.Spec.Rules[ruleIndex]
	host := r.Host
	ingresPath := ingress.Spec.Rules[ruleIndex].HTTP.Paths[pathIndex]

	ALBSSLAnnotation := fmt.Sprintf("alb.networking.%s/tls", c.GetDomain())

	ingInfo := fmt.Sprintf("%s/%s:%v:%v", ingress.Namespace, ingress.Name, ft.Spec.Port, pathIndex)

	annotations := ingress.GetAnnotations()

	rewriteTarget := annotations[ALBRewriteTargetAnnotation]

	vhost := annotations[ALBVHostAnnotation]

	enableCORS := annotations[ALBEnableCORSAnnotation] == "true"
	corsAllowHeaders := annotations[ALBCORSAllowHeadersAnnotation]
	corsAllowOrigin := annotations[ALBCORSAllowOriginAnnotation]

	backendProtocol := strings.ToLower(annotations[ALBBackendProtocolAnnotation])

	var priority int = DefaultPriority
	priorityKey := fmt.Sprintf(FMT_ALBRulePriorityAnnotation, c.Domain, ruleIndex, pathIndex)
	if annotations[priorityKey] != "" {
		priority64, err := strconv.ParseInt(annotations[priorityKey], 10, 64)
		if err != nil {
			c.log.Error(err, "parse priority fail", "ing", ingInfo, "priority", annotations[priorityKey])
		}
		if priority64 != 0 {
			priority = int(priority64)
		}
	}

	// rule-ext redirect
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

	ruleAnnotation := map[string]string{}

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
	// ingress version could be change frequently, when OwnerReference change or status change. we should not record it.
	ruleAnnotation[c.GetLabelSourceIngressPathIndex()] = fmt.Sprintf("%d", pathIndex)
	ruleAnnotation[c.GetLabelSourceIngressRuleIndex()] = fmt.Sprintf("%d", ruleIndex)
	name := strings.ToLower(utils.RandomStr(ft.Name, 4))
	dslx := GetDSLX(host, url, pathType)
	ruleSpec := alb2v1.RuleSpec{
		Domain:           host,
		URL:              url,
		DSL:              dslx.ToSearchableString(),
		DSLX:             dslx,
		Priority:         priority, // TODO ability to set priority in ingress.
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
		Config:           &alb2v1.RuleConfigInCr{},
		ServiceGroup: &alb2v1.ServiceGroup{ // TODO ingress could only have one service as backend
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

	cfg := c
	ruleRes := &alb2v1.Rule{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   ft.Namespace,
			Annotations: ruleAnnotation,
			Labels: map[string]string{
				cfg.GetLabelAlbName():    albKey.Name,
				cfg.GetLabelFt():         ft.Name,
				cfg.GetLabelSourceType(): m.TypeIngress,
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
	if project != "" {
		ruleRes.Labels[cfg.GetLabelProject()] = project
	}
	// in most case 63 is enough, we use those label only manually
	ruleRes.Labels[cfg.GetLabelSourceName()] = CutSize(ingress.Name, 63)
	ruleRes.Labels[cfg.GetLabelSourceNs()] = CutSize(ingress.Namespace, 63)
	ruleRes.Labels[cfg.GetLabelSourceIndex()] = fmt.Sprintf("%d-%d", ruleIndex, pathIndex)
	c.cus.IngressAnnotationToRule(ingress, ruleIndex, pathIndex, ruleRes)
	return ruleRes, nil
}

func CutSize(s string, size int) string {
	if len(s) > size {
		return s[:size]
	}
	return s
}

type SyncRule struct {
	Create []*alb2v1.Rule
	Delete []*alb2v1.Rule
	Update []*alb2v1.Rule
}

func (r *SyncRule) needDo() bool {
	if r == nil {
		return false
	}
	return len(r.Create) != 0 || len(r.Delete) != 0 || len(r.Update) != 0
}

func (c *Controller) doUpdateRule(r *SyncRule) error {
	if !r.needDo() {
		return nil
	}

	// we want create rule first. since that if this pod crash, it will not loss rule at least.
	c.log.Info("update rule", "create", len(r.Create), "delete", len(r.Delete), "update", len(r.Update))
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
		c.log.Info("delete rule ok", "rule", r.Name, "rule", r)
	}

	for _, r := range r.Update {
		nr, err := c.kd.UpdateRule(r)
		if err != nil {
			return err
		}
		c.log.Info("rule update", "dff", cmp.Diff(r, nr))
	}
	return nil
}

func (c *Controller) genSyncRuleAction(kind string, ing *networkingv1.Ingress, existRules []*alb2v1.Rule, expectRules []*alb2v1.Rule, log logr.Logger) (*SyncRule, error) {
	log = log.WithValues("kind", kind, "ing", ing.Name, "ing-ver", ing.ResourceVersion)
	log.Info("check update rule", "exist-rule-len", len(existRules), "expect-rule-len", len(expectRules))

	showRule := func(r *alb2v1.Rule) []interface{} {
		indexDesc := c.GenRuleIndex(r)
		pathDesc := ""
		{
			p, err := c.AccessIngressPathViaRule(ing, r)
			if err != nil {
				pathDesc = fmt.Sprintf("%v", err)
			} else {
				pathDesc = p.Path
			}
		}
		tags := []interface{}{"name", r.Name, "ingress", r.Spec.Source.Name, "index", indexDesc, "path", pathDesc}
		if c.DebugRuleSync() {
			tags = append(tags, "id", ruleIdentify(r))
		}
		return tags
	}
	_ = showRule

	// need delete
	needDelete := []*alb2v1.Rule{}
	needCreate := []*alb2v1.Rule{}
	needUpdate := []*alb2v1.Rule{}
	existRuleHash := map[string]bool{}
	expectRuleHash := map[string]bool{}
	existRuleIndex := map[string]*alb2v1.Rule{}
	expectRuleIndex := map[string]*alb2v1.Rule{}
	// we need a layered map  ingress/ver/rule-index/path-index/exist|expect/hash: v
	for _, r := range expectRules {
		hash := ruleHash(r)
		index := c.GenRuleIndex(r)
		log.Info("expect rule ", "name", r.Name, "index", index)
		expectRuleHash[hash] = true
		expectRuleIndex[index] = r
	}

	for _, r := range existRules {
		hash := ruleHash(r)
		index := c.GenRuleIndex(r)
		log.Info("exist rule ", "name", r.Name, "index", index)
		existRuleHash[hash] = true
		if er, find := existRuleIndex[index]; !find {
			existRuleIndex[index] = r
		} else {
			log.Info("find same index rule ", "index", index, "er", er.Name, "r", r.Name)
			needDelete = append(needDelete, r)
		}
	}

	for _, r := range existRules {
		index := c.GenRuleIndex(r)
		if _, ok := expectRuleIndex[index]; !ok {
			log.Info("need delete rule ", "name", r.Name, "index", index, "cr", utils.PrettyJson(r))
			needDelete = append(needDelete, r)
		}
	}

	for _, r := range expectRules {
		index := c.GenRuleIndex(r)
		if _, ok := existRuleIndex[index]; !ok {
			log.Info("need create rule ", "name", r.Name, "cr", utils.PrettyJson(r))
			needCreate = append(needCreate, r)
		}
	}

	for _, r := range expectRules {
		index := c.GenRuleIndex(r)
		if er, ok := existRuleIndex[index]; ok {
			hash := ruleHash(r)
			if hash != hashRule(ruleIdentify(er)) {
				r.Name = er.Name
				r.ResourceVersion = er.ResourceVersion
				log.Info("need update rule ", "name", r.Name, "diff", cmp.Diff(er, r))
				needUpdate = append(needUpdate, r)
			}
		}
	}

	if len(needCreate) == 0 && len(needDelete) == 0 && len(needUpdate) == 0 {
		log.Info("nothing need todo ingress synced.")
	}

	for _, r := range needCreate {
		log.Info("should create rule", showRule(r)...)
	}
	for _, r := range needDelete {
		log.Info("should delete rule", showRule(r)...)
	}
	for _, r := range needDelete {
		log.Info("should update rule", showRule(r)...)
	}

	// only when ingress add or remove a path, otherwise we do not create/delete rule
	return &SyncRule{
		Create: needCreate,
		Delete: needDelete,
		Update: needUpdate,
	}, nil
}

// rule which have same identity considered as same rule
// we may add some label/annotation in rule,such as creator/update time, etc
// we do not care about ingress resource version
// if two rule has same identify,they are same.
func ruleIdentify(r *alb2v1.Rule) string {
	label := []string{}
	annotation := []string{}
	spec := r.Spec
	// do care about the other hash
	for k, v := range r.Labels {
		if strings.HasPrefix(k, "alb") {
			label = append(label, k+":"+v)
		}
		if k == "cpaas.io/project" {
			label = append(label, k+":"+v)
		}
	}
	for k, v := range r.Annotations {
		if strings.HasPrefix(k, "alb") {
			annotation = append(annotation, k+":"+v)
		}
		if strings.HasPrefix(k, "nginx.ingress") {
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
	return b.String()
}

func ruleHash(r *alb2v1.Rule) string {
	return hashRule(ruleIdentify(r))
}

func hashRule(id string) string {
	h := sha256.New()
	h.Write([]byte(id))
	return hex.EncodeToString(h.Sum(nil))
}

func GetDSLX(domain, url string, pathType networkingv1.PathType) alb2v1.DSLX {
	var dslx alb2v1.DSLX
	if url != "" {
		switch pathType {
		case networkingv1.PathTypeExact:
			dslx = append(dslx, alb2v1.DSLXTerm{
				Values: [][]string{
					{utils.OP_EQ, url},
				},
				Type: utils.KEY_URL,
			})
		case networkingv1.PathTypePrefix:
			dslx = append(dslx, alb2v1.DSLXTerm{
				Values: [][]string{
					{utils.OP_STARTS_WITH, url},
				},
				Type: utils.KEY_URL,
			})
		default:
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

func (c *Controller) GenRuleIndex(rule *alb2v1.Rule) string {
	rindex := c.GetLabelSourceIngressRuleIndex()
	pindex := c.GetLabelSourceIngressPathIndex()
	return fmt.Sprintf("%s:%s", rule.Annotations[rindex], rule.Annotations[pindex])
}
