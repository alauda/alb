/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ingress

import (
	informerv1 "alauda.io/alb2/pkg/client/informers/externalversions/alauda/v1"
	listerv1 "alauda.io/alb2/pkg/client/listers/alauda/v1"

	"context"
	"fmt"
	"github.com/thoas/go-funk"
	"k8s.io/apimachinery/pkg/labels"
	"os"
	"reflect"
	"strings"
	"time"

	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	networkingv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	networkinginformers "k8s.io/client-go/informers/extensions/v1beta1"
	scheme "k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	networkinglisters "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"alauda.io/alb2/config"
	ctl "alauda.io/alb2/controller"
	"alauda.io/alb2/driver"
	m "alauda.io/alb2/modules"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
)

func (c *Controller) Start(ctx context.Context) {
	interval := config.GetInt("INTERVAL")

	wait.PollInfinite(time.Duration(interval)*time.Second, func() (bool, error) {
		isLeader, err := ctl.IsLocker(c.KubernetesDriver)
		if err != nil {
			klog.Errorf("not leader: %s", err.Error())
			return false, nil
		}
		if isLeader {
			return true, nil
		}
		return false, nil
	})

	resyncPeriod := time.Duration(config.GetInt("RESYNC_PERIOD")) * time.Second
	klog.Infof("start periodicity sync with period: %s", resyncPeriod)
	go wait.Forever(func() {
		klog.Info("doing a periodicity sync")
		isLeader, err := ctl.IsLocker(c.KubernetesDriver)
		if err != nil || !isLeader {
			klog.Warningf("not leader, skip periodicity sync")
			return
		}
		rules, err := c.ruleLister.Rules(config.Get("NAMESPACE")).List(labels.SelectorFromSet(map[string]string{
			fmt.Sprintf("alb2.%s/source-type", config.Get("DOMAIN")):     "ingress",
			fmt.Sprintf(config.Get("labels.name"), config.Get("DOMAIN")): config.Get("NAME"),
		}))
		if err != nil {
			klog.Warningf("failed list rules: %v", err)
			return
		}
		httpProcessedIngress := make(map[string]bool)
		httpsProcessedIngress := make(map[string]bool)
		for _, rl := range rules {
			ingKey := rl.Labels[fmt.Sprintf("alb2.%s/source-name", config.Get("DOMAIN"))]
			var proto string
			if strings.Contains(rl.Name, fmt.Sprintf("%s-%05d", config.Get("NAME"), IngressHTTPPort)) {
				proto = m.ProtoHTTP
			} else if strings.Contains(rl.Name, fmt.Sprintf("%s-%05d", config.Get("NAME"), IngressHTTPSPort)) {
				proto = m.ProtoHTTPS
			}
			if ingKey != "" {
				ingNs := ingKey[strings.LastIndex(ingKey, ".")+1:]
				ingName := ingKey[:strings.LastIndex(ingKey, ".")]
				if _, err := c.ingressLister.Ingresses(ingNs).Get(ingName); err != nil {
					if errors.IsNotFound(err) {
						klog.Infof("ingress %s/%s not exist, remove rule %s/%s", ingNs, ingName, rl.Namespace, rl.Name)
						c.KubernetesDriver.ALBClient.CrdV1().Rules(rl.Namespace).Delete(rl.Name, &metav1.DeleteOptions{})
					}
				} else {
					if proto == m.ProtoHTTP {
						httpProcessedIngress[ingNs+"/"+ingName] = true
					} else if proto == m.ProtoHTTPS {
						httpsProcessedIngress[ingNs+"/"+ingName] = true
					}
				}
			}
		}
		ings, err := c.ingressLister.Ingresses("").List(labels.Everything())
		if err != nil {
			klog.Warningf("failed list ingress: %v", err)
			return
		}
		for _, ing := range ings {
			needFtTypes := getIngressFtTypes(ing)
			if needFtTypes.Has(m.ProtoHTTP) && !httpProcessedIngress[ing.Namespace+"/"+ing.Name] {
				if c.needEnqueueObject(ing) {
					c.enqueue(ing)
				}
			} else if needFtTypes.Has(m.ProtoHTTPS) && !httpsProcessedIngress[ing.Namespace+"/"+ing.Name] {
				if c.needEnqueueObject(ing) {
					c.enqueue(ing)
				}
			}
		}
		klog.Info("finish a periodicity sync")
	}, resyncPeriod)

	if err := c.Run(1, ctx.Done()); err != nil {
		klog.Errorf("Error running controller: %s", err.Error())
	}
}

// Controller is the controller implementation for Foo resources
type Controller struct {
	ingressLister   networkinglisters.IngressLister
	ruleLister      listerv1.RuleLister
	namespaceLister corelisters.NamespaceLister

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder

	KubernetesDriver *driver.KubernetesDriver
}

// NewController returns a new sample controller
func NewController(
	d *driver.KubernetesDriver,
	alb2Informer informerv1.ALB2Informer,
	ruleInformer informerv1.RuleInformer,
	ingressInformer networkinginformers.IngressInformer,
	namespaceLister corelisters.NamespaceLister) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	klog.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{Interface: d.Client.CoreV1().Events("")},
	)
	hostname, _ := os.Hostname()
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		corev1.EventSource{Component: fmt.Sprintf("alb2-%s", config.Get("NAME")), Host: hostname},
	)

	controller := &Controller{
		ingressLister:   ingressInformer.Lister(),
		ruleLister:      ruleInformer.Lister(),
		namespaceLister: namespaceLister,
		workqueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			"Ingresses",
		),
		recorder:         recorder,
		KubernetesDriver: d,
	}

	klog.Info("Setting up event handlers")

	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			newIngress := obj.(*networkingv1beta1.Ingress)
			klog.Infof("receive ingress %s/%s create event", newIngress.Namespace, newIngress.Name)
			controller.handleObject(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			newIngress := new.(*networkingv1beta1.Ingress)
			oldIngress := old.(*networkingv1beta1.Ingress)
			if newIngress.ResourceVersion == oldIngress.ResourceVersion {
				return
			}
			if reflect.DeepEqual(newIngress.Annotations, oldIngress.Annotations) && reflect.DeepEqual(newIngress.Spec, oldIngress.Spec) {
				return
			}
			klog.Infof("receive ingress %s/%s update event", newIngress.Namespace, newIngress.Name)
			controller.handleObject(new)
		},
		DeleteFunc: func(obj interface{}) {
			oldIngress := obj.(*networkingv1beta1.Ingress)
			klog.Infof("receive ingress %s/%s delete event", oldIngress.Namespace, oldIngress.Name)
			controller.handleObject(obj)
		},
	})
	alb2Informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newAlb2 := new.(*alb2v1.ALB2)
			oldAlb2 := old.(*alb2v1.ALB2)
			if oldAlb2.ResourceVersion == newAlb2.ResourceVersion {
				return
			}
			if reflect.DeepEqual(oldAlb2.Labels, newAlb2.Labels) {
				return
			}
			newProjects := ctl.GetOwnProjects(newAlb2.Name, newAlb2.Labels)
			oldProjects := ctl.GetOwnProjects(oldAlb2.Name, oldAlb2.Labels)
			newAddProjects := funk.SubtractString(newProjects, oldProjects)
			klog.Infof("own projects: %v, new add projects: %v", newProjects, newAddProjects)
			ingresses := controller.GetProjectIngresses(newAddProjects)
			for _, ingress := range ingresses {
				controller.handleObject(ingress)
			}
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("start ingress controller")
	klog.Info("Starting workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	klog.Info("Started workers")
	ings, err := c.ingressLister.Ingresses("").List(labels.Everything())
	if err != nil {
		klog.Error("error list all ingresses", err)
	} else {
		// ensure legacy ingresses will be transformed
		for _, ing := range ings {
			if c.needEnqueueObject(ing) {
				klog.Infof("enqueue unprocessed ing: %s/%s", ing.Namespace, ing.Name)
				c.enqueue(ing)
			}
		}
	}
	<-stopCh
	klog.Info("Shutting down workers")

	return nil
}

func (c *Controller) enqueue(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	klog.Infof("enqueue %s", key)
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Foo resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Foo resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	isLeader, err := ctl.IsLocker(c.KubernetesDriver)
	if err != nil {
		klog.Errorf("not leader: %s", err.Error())
		return
	}
	if !isLeader {
		return
	}
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			err := fmt.Errorf("error decoding object, invalid type")
			klog.Error(err)
			runtime.HandleError(err)
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		klog.Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	klog.Infof("Processing object: %s", object.GetName())
	if c.needEnqueueObject(object) {
		c.enqueue(object)
	}
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		klog.Error(err)
		return true
	}

	return true
}

func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	klog.Infof("Sync ingress %s/%s", namespace, name)
	ingress, err := c.ingressLister.Ingresses(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return c.onIngressDelete(name, namespace)
		}
		klog.Errorf("Handle %s.%s failed: %s", name, namespace, err.Error())
		return err
	}
	klog.Infof("Process ingress %s.%s, %+v", ingress.Name, ingress.Namespace, ingress)
	err = c.onIngressCreateOrUpdate(ingress)
	if err != nil {
		return err
	}
	c.recorder.Event(ingress, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)

	return nil
}

// setFtDefault will set ingress default backend as frontend default service group
func (c *Controller) setFtDefault(ingress *networkingv1beta1.Ingress, ft *m.Frontend) bool {
	if ingress.Spec.Backend == nil {
		return false
	}
	annotations := ingress.GetAnnotations()
	backendProtocol := strings.ToLower(annotations[ALBBackendProtocolAnnotation])

	needSave := false
	if ft.ServiceGroup == nil ||
		len(ft.ServiceGroup.Services) == 0 {
		ft.ServiceGroup = &alb2v1.ServiceGroup{
			Services: []alb2v1.Service{
				alb2v1.Service{
					Namespace: ingress.Namespace,
					Name:      ingress.Spec.Backend.ServiceName,
					Port:      int(ingress.Spec.Backend.ServicePort.IntVal),
					Weight:    100,
				},
			},
		}
		ft.BackendProtocol = backendProtocol
		ft.Source = &alb2v1.Source{
			Type:      m.TypeIngress,
			Name:      ingress.Name,
			Namespace: ingress.Namespace,
		}
		needSave = true
	} else {
		// ft has default service
		if !ft.ServiceGroup.Services[0].Is(
			ingress.Namespace,
			ingress.Spec.Backend.ServiceName,
			int(ingress.Spec.Backend.ServicePort.IntVal)) {
			klog.Warningf("frontend %s already has default service, conflict", ft.Name)
			//TODO Add event here
		}
	}
	return needSave
}

// updateRule update or create rules for a ingress
func (c *Controller) updateRule(
	ingress *networkingv1beta1.Ingress,
	ft *m.Frontend,
	host string,
	ingresPath networkingv1beta1.HTTPIngressPath,
) error {
	ingInfo := fmt.Sprintf("%s/%s", ingress.Namespace, ingress.Name)

	annotations := ingress.GetAnnotations()
	rewriteTarget := annotations[ALBRewriteTargetAnnotation]
	vhost := annotations[ALBVHostAnnotation]
	enableCORS := annotations[ALBEnableCORSAnnotation] == "true"
	backendProtocol := strings.ToLower(annotations[ALBBackendProtocolAnnotation])
	var (
		redirectURL  string
		redirectCode int
	)
	if annotations[ALBPermanentRedirectAnnotation] != "" && annotations[ALBTemporalRedirectAnnotation] != "" {
		klog.Errorf("cannot use PermanentRedirect and TemporalRedirect at same time, ingress %s", ingInfo)
		return nil
	}
	if annotations[ALBPermanentRedirectAnnotation] != "" {
		redirectURL = annotations[ALBPermanentRedirectAnnotation]
		redirectCode = 301
	}
	if annotations[ALBTemporalRedirectAnnotation] != "" {
		redirectURL = annotations[ALBTemporalRedirectAnnotation]
		redirectCode = 302
	}
	certs := make(map[string]string)

	if backendProtocol != "" && !ValidBackendProtocol[backendProtocol] {
		klog.Errorf("Unsupported backend protocol %s for ingress %s", backendProtocol, ingInfo)
		return nil
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

	url := ingresPath.Path
	for _, rule := range ft.Rules {
		if rule.Source == nil {
			// user defined rule
			continue
		}
		if rule.Source.Type == m.TypeIngress &&
			rule.Source.Name == ingress.Name &&
			rule.Source.Namespace == ingress.Namespace &&
			strings.ToLower(rule.Domain) == host &&
			rule.URL == url &&
			rule.RewriteBase == url &&
			rule.RewriteTarget == rewriteTarget &&
			rule.CertificateName == certs[host] &&
			rule.EnableCORS == enableCORS &&
			rule.BackendProtocol == backendProtocol &&
			rule.RedirectURL == redirectURL &&
			rule.RedirectCode == redirectCode &&
			rule.VHost == vhost {
			// already have

			// FIX: http://jira.alaudatech.com/browse/DEV-16951
			if rule.ServiceGroup != nil {
				found := false
				for _, svc := range rule.ServiceGroup.Services {
					if svc.Name == ingresPath.Backend.ServiceName {
						found = true
						break
					}
				}
				if !found {
					// when add a new service to service group we need to re calculate weight
					newsvc := alb2v1.Service{
						Namespace: ingress.Namespace,
						Name:      ingresPath.Backend.ServiceName,
						Port:      int(ingresPath.Backend.ServicePort.IntVal),
						Weight:    100,
					}
					rule.ServiceGroup.Services = append(rule.ServiceGroup.Services, newsvc)
					weight := 100 / len(rule.ServiceGroup.Services)
					newServices := []alb2v1.Service{}
					for _, svc := range rule.ServiceGroup.Services {
						svc.Weight = weight
						newServices = append(newServices, svc)
					}
					rule.ServiceGroup.Services = newServices
				}
				err := c.KubernetesDriver.UpdateRule(rule)
				if err != nil {
					klog.Errorf(
						"update rule %+v for ingress %s failed: %s",
						*rule, ingInfo, err.Error(),
					)
					return err
				}
			}
			return nil
		}
	}
	rule, err := ft.NewRule(ingInfo, host, url, rewriteTarget, backendProtocol, certs[host], enableCORS, redirectURL, redirectCode, vhost, DefaultPriority)
	if err != nil {
		klog.Error(err)
		return err
	}
	rule.ServiceGroup = &alb2v1.ServiceGroup{
		Services: []alb2v1.Service{
			alb2v1.Service{
				Namespace: ingress.Namespace,
				Name:      ingresPath.Backend.ServiceName,
				Port:      int(ingresPath.Backend.ServicePort.IntVal),
				Weight:    100,
			},
		},
	}
	rule.Source = &alb2v1.Source{
		Type:      m.TypeIngress,
		Namespace: ingress.Namespace,
		Name:      ingress.Name,
	}
	err = c.KubernetesDriver.CreateRule(rule)
	if err != nil {
		klog.Errorf(
			"Create rule %+v for ingress %s failed: %s",
			*rule, ingInfo, err.Error(),
		)
		return err
	}
	klog.Infof("Create rule %s for ingress %s success", rule.Name, ingInfo)
	return nil
}

func (c *Controller) onIngressCreateOrUpdate(ingress *networkingv1beta1.Ingress) error {
	klog.Infof("on ingress create or update, %s/%s", ingress.Namespace, ingress.Name)
	// Detele old rule if it exist
	c.onIngressDelete(ingress.Name, ingress.Namespace)

	// then create new one
	alb, err := c.KubernetesDriver.LoadALBbyName(
		config.Get("NAMESPACE"),
		config.Get("NAME"),
	)
	if err != nil {
		klog.Error(err)
		return err
	}

	defaultSSLCert := strings.ReplaceAll(config.Get("DEFAULT-SSL-CERTIFICATE"), "/", "_")
	sslMap := parseSSLAnnotation(ingress.Annotations[ALBSSLAnnotation])
	certs := make(map[string]string)
	for host, cert := range sslMap {
		if certs[strings.ToLower(host)] == "" {
			certs[strings.ToLower(host)] = cert
		}
	}

	needFtTypes := getIngressFtTypes(ingress)

	// default backend we will not create rules but save service to frontend's servicegroup
	isDefaultBackend := isDefaultBackend(ingress)
	klog.Infof("%s is default backend: %t", ingress.Name, isDefaultBackend)
	if isDefaultBackend {
		needFtTypes.Add(m.ProtoHTTP)
	}
	klog.Infof("needFtTypes, %s", needFtTypes.String())
	var httpFt *m.Frontend
	var httpsFt *m.Frontend
	for _, f := range alb.Frontends {
		if needFtTypes.Has(m.ProtoHTTP) && f.Port == IngressHTTPPort {
			httpFt = f
		}
		if needFtTypes.Has(m.ProtoHTTPS) && f.Port == IngressHTTPSPort {
			httpsFt = f
		}
	}
	if httpFt != nil && httpFt.Protocol != m.ProtoHTTP {
		err = fmt.Errorf("Port %d is not an HTTP port, protocol: %s", IngressHTTPPort, httpFt.Protocol)
		klog.Error(err)
		return err
	}
	if httpsFt != nil {
		if httpsFt.Protocol != m.ProtoHTTPS {
			err = fmt.Errorf("Port %d is not an HTTPS port, protocol: %s", IngressHTTPSPort, httpsFt.Protocol)
			klog.Error(err)
			return err
		}
		if httpsFt.CertificateName != "" && httpsFt.CertificateName != defaultSSLCert {
			klog.Warningf("Port %d already has ssl cert conflict with default ssl cert", IngressHTTPSPort)
		}
	}

	newHTTPFrontend := false
	newHTTPSFrontend := false

	if httpFt == nil {
		httpFt, err = alb.NewFrontend(IngressHTTPPort, m.ProtoHTTP, "")
		if err != nil {
			klog.Error(err)
			return err
		}
		newHTTPFrontend = true
	}
	if httpsFt == nil && !isDefaultBackend {
		httpsFt, err = alb.NewFrontend(IngressHTTPSPort, m.ProtoHTTPS, defaultSSLCert)
		if err != nil {
			klog.Error(err)
			return err
		}
		newHTTPSFrontend = true
	}

	needSaveHTTP := false
	if isDefaultBackend {
		needSaveHTTP = c.setFtDefault(ingress, httpFt)
		klog.Infof("need save default service for ft: %s, %t", httpFt.Name, needSaveHTTP)
	}
	// make sure we have a fronted before we create rules
	if needFtTypes.Has(m.ProtoHTTP) && (newHTTPFrontend || needSaveHTTP) {
		err = c.KubernetesDriver.UpsertFrontends(alb, httpFt)
		if err != nil {
			klog.Errorf("upsert ft failed: %s", err)
			return err
		}
	}
	if needFtTypes.Has(m.ProtoHTTPS) {
		needUpdate := false
		if httpsFt.CertificateName == "" && defaultSSLCert != "" {
			needUpdate = true
		}
		if newHTTPSFrontend || needUpdate {
			err = c.KubernetesDriver.UpsertFrontends(alb, httpsFt)
			if err != nil {
				klog.Errorf("upsert ft failed: %s", err)
				return err
			}
		}
	}

	// create rules
	for _, r := range ingress.Spec.Rules {
		host := strings.ToLower(r.Host)
		httpRules := r.IngressRuleValue.HTTP
		if httpRules == nil {
			klog.Infof("No http rule found on ingress %s/%s under host %s.",
				ingress.Namespace,
				ingress.Name,
				host,
			)
			continue
		}
		for _, p := range httpRules.Paths {
			for _, proto := range []string{m.ProtoHTTPS, m.ProtoHTTP} {
				if needFtTypes.Has(proto) {
					var ft *m.Frontend
					if proto == m.ProtoHTTP && httpFt != nil {
						ft = httpFt
					} else if proto == m.ProtoHTTPS && httpsFt != nil {
						ft = httpsFt
					} else {
						continue
					}
					err = c.updateRule(ingress, ft, host, p)
					if err != nil {
						klog.Errorf(
							"Update %s rule failed for ingress %s/%s with host=%s, path=%s",
							proto,
							ingress.Namespace,
							ingress.Name,
							host,
							p.Path,
						)
						return err
					} else {
						klog.Infof(
							"Update %s rule success for ingress %s/%s with host=%s, path=%s",
							proto,
							ingress.Namespace,
							ingress.Name,
							host,
							p.Path,
						)
					}
				}
			}
		}
	}
	return nil
}

func (c *Controller) onIngressDelete(name, namespace string) error {
	klog.Infof("on ingress delete, %s/%s", namespace, name)
	alb, err := c.KubernetesDriver.LoadALBbyName(
		config.Get("NAMESPACE"),
		config.Get("NAME"),
	)
	if err != nil {
		klog.Error(err)
		return err
	}
	var ft *m.Frontend
	for _, f := range alb.Frontends {
		if f.Port == IngressHTTPPort || f.Port == IngressHTTPSPort {
			ft = f
			if ft.Source != nil &&
				ft.Source.Type == m.TypeIngress &&
				ft.Source.Namespace == namespace &&
				ft.Source.Name == name {
				ft.ServiceGroup = nil
				ft.Source = nil
				ft.BackendProtocol = ""
				err = c.KubernetesDriver.UpsertFrontends(alb, ft)
				if err != nil {
					klog.Errorf("upsert ft failed: %s", err)
					return err
				}
			}

			for _, rule := range ft.Rules {
				if rule.Source != nil &&
					rule.Source.Type == m.TypeIngress &&
					rule.Source.Namespace == namespace &&
					rule.Source.Name == name {

					err = c.KubernetesDriver.DeleteRule(rule)
					if err != nil {
						klog.Errorf("upsert ft failed: %s", err)
						return err
					}
				}
			}
		}
	}
	return nil
}

func (c *Controller) needEnqueueObject(obj metav1.Object) (rv bool) {
	klog.Infof("check if ingress %s/%s need enqueue", obj.GetNamespace(), obj.GetName())
	defer func() {
		klog.Infof("check ingress %s/%s result: %t", obj.GetNamespace(), obj.GetName(), rv)
	}()
	annotations := obj.GetAnnotations()
	if !(annotations["kubernetes.io/ingress.class"] == "" ||
		annotations["kubernetes.io/ingress.class"] == config.Get("NAME")) {
		return false
	}
	alb, err := c.KubernetesDriver.LoadALBbyName(config.Get("NAMESPACE"), config.Get("NAME"))
	if err != nil {
		klog.Errorf("get alb res failed, %+v", err)
		return false
	}
	belongProject := c.GetIngressBelongProject(obj)
	role := ctl.GetAlbRoleType(alb.Labels)
	if role == ctl.RolePort {
		hasHTTPPort := false
		hasHTTPSPort := false
		httpPortProjects := []string{}
		httpsPortProjects := []string{}
		for _, ft := range alb.Frontends {
			if ft.Port == IngressHTTPPort {
				hasHTTPPort = true
				httpPortProjects = ctl.GetOwnProjects(ft.Name, ft.Lables)
			} else if ft.Port == IngressHTTPSPort {
				hasHTTPSPort = true
				httpsPortProjects = ctl.GetOwnProjects(ft.Name, ft.Lables)
			}
			if hasHTTPSPort && hasHTTPPort {
				break
			}
		}
		// for role=port alb user should create http and https ports before using ingress
		if !(hasHTTPPort && hasHTTPSPort) {
			return false
		}
		if (funk.Contains(httpPortProjects, m.ProjectALL) || funk.Contains(httpPortProjects, belongProject)) &&
			(funk.Contains(httpsPortProjects, m.ProjectALL) || funk.Contains(httpsPortProjects, belongProject)) {
			return true
		}
	} else {
		projects := ctl.GetOwnProjects(alb.Name, alb.Labels)
		if funk.Contains(projects, m.ProjectALL) {
			return true
		}
		if funk.Contains(projects, belongProject) {
			return true
		}
	}
	return false
}

func (c *Controller) GetProjectIngresses(projects []string) []*networkingv1beta1.Ingress {
	if funk.ContainsString(projects, m.ProjectALL) {
		ingress, err := c.ingressLister.Ingresses("").List(labels.Everything())
		if err != nil {
			klog.Error(err)
			return nil
		}
		return ingress
	}
	var allIngresses []*networkingv1beta1.Ingress
	for _, project := range projects {
		sel := labels.Set{fmt.Sprintf("%s/project", config.Get("DOMAIN")): project}.AsSelector()
		nss, err := c.namespaceLister.List(sel)
		if err != nil {
			klog.Error(err)
			return nil
		}
		for _, ns := range nss {
			ingress, err := c.ingressLister.Ingresses(ns.Name).List(labels.Everything())
			if err != nil {
				klog.Error(err)
				return nil
			}
			allIngresses = append(allIngresses, ingress...)
		}
	}
	return allIngresses
}

func (c *Controller) GetIngressBelongProject(obj metav1.Object) string {
	if ns := obj.GetNamespace(); ns != "" {
		nsCr, err := c.namespaceLister.Get(ns)
		if err != nil {
			klog.Errorf("get namespace failed, %+v", err)
			return ""
		}
		domain := config.Get("DOMAIN")
		if project := nsCr.Labels[fmt.Sprintf("%s/project", domain)]; project != "" {
			return project
		}
	}
	return ""
}
