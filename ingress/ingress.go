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
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"alauda.io/alb2/config"
	ctl "alauda.io/alb2/controller"
	"alauda.io/alb2/driver"
	m "alauda.io/alb2/modules"
	alb2v1 "alauda.io/alb2/pkg/apis/alauda/v1"
	informerv1 "alauda.io/alb2/pkg/client/informers/externalversions/alauda/v1"
	listerv1 "alauda.io/alb2/pkg/client/listers/alauda/v1"
	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	networkinginformers "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	networkinglisters "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

func (c *Controller) Start(ctx context.Context) {
	ctl.WaitUtilIMLeader(ctx, c.KubernetesDriver)

	resyncPeriod := time.Duration(config.GetInt("RESYNC_PERIOD")) * time.Second
	if config.GetBool("FULL_SYNC") {
		klog.Infof("start periodicity sync with period: %s", resyncPeriod)
		go wait.Forever(func() {

			isLeader, err := ctl.IsLocker(c.KubernetesDriver)
			if err != nil || !isLeader {
				klog.Warningf("not leader, skip periodicity sync")
				return
			}

			klog.Info("doing a periodicity sync")
			ingressList, err := c.findUnSyncedIngress(ctx)
			if err != nil {
				klog.Errorf("find unsynced ingress fail: %v", err)
				return
			}
			count := 0
			for _, ing := range ingressList {
				if c.needEnqueueObject(ing, true) {
					count++
					klog.Infof("find-unsync: get a unsyncd ingress resync it %v %v %v", ing.Name, ing.Namespace, ing.ResourceVersion)
					c.enqueue(ing)
				}
			}
			klog.Infof("find-unsync unsynced-ingress-len: %d", count)

		}, resyncPeriod)
	} else {
		klog.Infof("full sync disabled by config")
	}

	if err := c.Run(1, ctx.Done()); err != nil {
		klog.Errorf("Error running controller: %s", err.Error())
	}
}

func (c *Controller) findUnSyncedIngress(ctx context.Context) ([]*networkingv1.Ingress, error) {

	IngressHTTPPort := config.GetInt("INGRESS_HTTP_PORT")
	IngressHTTPSPort := config.GetInt("INGRESS_HTTPS_PORT")

	ingressList := make([]*networkingv1.Ingress, 0)
	sel := labels.SelectorFromSet(map[string]string{
		config.GetLabelSourceType(): m.TypeIngress,
		config.GetLabelAlbName():    config.GetAlbName(),
	})
	rules, err := c.ruleLister.Rules(config.Get("NAMESPACE")).List(sel)
	if err != nil {
		klog.Errorf("failed list rules: %v", err)
		return ingressList, err
	}

	klog.Infof("find-unsync rules-len: %d sel %v", len(rules), sel)

	httpProcessedIngress := make(map[string]string)
	httpsProcessedIngress := make(map[string]string)
	for _, rl := range rules {
		if rl.Spec.Source == nil || rl.Spec.Source.Type != m.TypeIngress {
			klog.Errorf("ingress rule but type is not ingress %s %+v", rl.Name, rl.Spec.Source)
			continue
		}
		var proto alb2v1.FtProtocol
		if strings.Contains(rl.Name, fmt.Sprintf("%s-%05d", config.Get("NAME"), IngressHTTPPort)) {
			proto = m.ProtoHTTP
		} else if strings.Contains(rl.Name, fmt.Sprintf("%s-%05d", config.Get("NAME"), IngressHTTPSPort)) {
			proto = m.ProtoHTTPS
		}
		ingNs := rl.Spec.Source.Namespace
		ingName := rl.Spec.Source.Name

		if _, err := c.ingressLister.Ingresses(ingNs).Get(ingName); err != nil {
			if errors.IsNotFound(err) {
				klog.Infof("ingress %s/%s not exist, remove rule %s/%s", ingNs, ingName, rl.Namespace, rl.Name)
				c.KubernetesDriver.ALBClient.CrdV1().Rules(rl.Namespace).Delete(ctx, rl.Name, metav1.DeleteOptions{})
			}
		} else {
			sourceIngressVersion := config.GetLabelSourceIngressVersion()
			if proto == m.ProtoHTTP {
				httpProcessedIngress[ingNs+"/"+ingName] = rl.Annotations[sourceIngressVersion]
			} else if proto == m.ProtoHTTPS {
				httpsProcessedIngress[ingNs+"/"+ingName] = rl.Annotations[sourceIngressVersion]
			}
		}
	}

	ings, err := c.ingressLister.Ingresses("").List(labels.Everything())
	if err != nil {
		klog.Warningf("failed list ingress: %v", err)
		return ingressList, err
	}

	for _, ing := range ings {
		needFtTypes := getIngressFtTypes(ing)
		need := false
		if needFtTypes.Has(m.ProtoHTTP) && !isIngressAlreadySynced(ing, httpProcessedIngress) {
			need = true
		} else if needFtTypes.Has(m.ProtoHTTPS) && !isIngressAlreadySynced(ing, httpsProcessedIngress) {
			need = true
		}
		if need {
			ingressList = append(ingressList, ing)
		}
	}

	return ingressList, nil
}

func isIngressAlreadySynced(ing *networkingv1.Ingress, processedIngress map[string]string) bool {
	ingKey := ing.Namespace + "/" + ing.Name
	if version, ok := processedIngress[ingKey]; ok {
		if version == ing.ResourceVersion {
			return true
		} else {
			klog.Infof("detect ingress %s change rule-v: %s ing-v: %s", ing.Name, version, ing.ResourceVersion)
			return false
		}
	} else {
		klog.Infof("detect ingress %s change rule-v: not exist ing-v: %s ", ing.Name, ing.ResourceVersion)
		return false
	}
}

// Controller is the controller implementation for Foo resources
type Controller struct {
	ingressLister         networkinglisters.IngressLister
	ingressClassAllLister networkinglisters.IngressClassLister
	ingressClassLister    ingressClassLister
	ruleLister            listerv1.RuleLister
	namespaceLister       corelisters.NamespaceLister

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

	// configuration for ingressClass
	icConfig *config.IngressClassConfiguration
}

// NewController returns a new sample controller
func NewController(
	d *driver.KubernetesDriver,
	alb2Informer informerv1.ALB2Informer,
	ruleInformer informerv1.RuleInformer,
	ingressInformer networkinginformers.IngressInformer,
	ingressClassInformer networkinginformers.IngressClassInformer,
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

	domain := config.Get("DOMAIN")
	ControllerName := fmt.Sprintf("%s/%s", domain, config.DefaultControllerName)

	controller := &Controller{
		ingressLister:         ingressInformer.Lister(),
		ingressClassAllLister: ingressClassInformer.Lister(),
		ingressClassLister:    ingressClassLister{cache.NewStore(cache.MetaNamespaceKeyFunc)},
		ruleLister:            ruleInformer.Lister(),
		namespaceLister:       namespaceLister,
		workqueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			"Ingresses",
		),
		recorder:         recorder,
		KubernetesDriver: d,

		icConfig: &config.IngressClassConfiguration{
			Controller:         ControllerName,
			AnnotationValue:    config.DefaultControllerName,
			WatchWithoutClass:  true,
			IgnoreIngressClass: false,
			IngressClassByName: true,
		},
	}
	if config.GetBool("INCREMENT_SYNC") {
		klog.Info("Setting up event handlers")
		controller.setUpEventHandler(alb2Informer, ingressInformer, ingressClassInformer)
	} else {
		klog.Infof("increment sync disabled by config")
	}

	return controller
}

func (c *Controller) setUpEventHandler(alb2Informer informerv1.ALB2Informer,
	ingressInformer networkinginformers.IngressInformer, ingressClassInformer networkinginformers.IngressClassInformer) {

	// icConfig: configuration items for ingressClass
	icConfig := c.icConfig

	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			newIngress := obj.(*networkingv1.Ingress)
			klog.Infof("receive ingress %s/%s %s create event", newIngress.Namespace, newIngress.Name, newIngress.ResourceVersion)
			ic, err := c.GetIngressClass(newIngress, icConfig)
			if err != nil {
				klog.InfoS("Ignoring ingress because of error while validating ingress class", "ingress", klog.KObj(newIngress), "error", err)
				return
			}
			klog.InfoS("Found valid IngressClass", "ingress", klog.KObj(newIngress), "ingressClass", ic)
			c.handleObject(newIngress)
		},
		UpdateFunc: func(old, new interface{}) {
			var errOld, errCur error
			var classCur string
			newIngress := new.(*networkingv1.Ingress)
			oldIngress := old.(*networkingv1.Ingress)
			_, errOld = c.GetIngressClass(oldIngress, icConfig)
			classCur, errCur = c.GetIngressClass(newIngress, icConfig)

			if newIngress.ResourceVersion == oldIngress.ResourceVersion {
				return
			}
			if reflect.DeepEqual(newIngress.Annotations, oldIngress.Annotations) && reflect.DeepEqual(newIngress.Spec, oldIngress.Spec) {
				return
			}
			klog.Infof("receive ingress %s/%s update event  version:%s/%s", newIngress.Namespace, newIngress.Name, oldIngress.ResourceVersion, newIngress.ResourceVersion)

			if errOld != nil && errCur == nil {
				klog.InfoS("creating ingress", "ingress", klog.KObj(newIngress), "ingressClass", classCur)
			} else if errOld == nil && errCur != nil {
				klog.InfoS("removing ingress because of ingressClass changed to other ingress controller", "ingress", klog.KObj(newIngress))
				c.onIngressDelete(oldIngress.Name, oldIngress.Namespace)
				return
			} else if errCur == nil && !reflect.DeepEqual(old, new) {
				klog.InfoS("update ingress", "ingress", klog.KObj(newIngress), "ingressClass", classCur)
			} else {
				return
			}
			c.handleObject(newIngress)
		},
		DeleteFunc: func(obj interface{}) {
			ingress := obj.(*networkingv1.Ingress)
			klog.Infof("receive ingress %s/%s %s delete event", ingress.Namespace, ingress.Name, ingress.ResourceVersion)
			_, err := c.GetIngressClass(ingress, icConfig)
			if err != nil {
				klog.InfoS("Ignoring ingress because of error while validating ingress class", "ingress", klog.KObj(ingress), "error", err)
				return
			}
			c.handleObject(ingress)
		},
	})

	ingressClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ingressClass := obj.(*networkingv1.IngressClass)
			if !CheckIngressClass(ingressClass, icConfig) {
				return
			}
			klog.InfoS("add new ingressClass related to the ingress controller", "ingressClass", klog.KObj(ingressClass))
			err := c.ingressClassLister.Add(ingressClass)
			if err != nil {
				klog.InfoS("error adding ingressClass to store", "ingressClass", klog.KObj(ingressClass), "error", err)
				return
			}
		},

		UpdateFunc: func(old, cur interface{}) {
			oic := old.(*networkingv1.IngressClass)
			cic := cur.(*networkingv1.IngressClass)
			if cic.Spec.Controller != icConfig.Controller {
				klog.InfoS("ignoring ingressClass as the spec.controller is not the same of this ingress", "ingressClass", klog.KObj(cic))
				return
			}
			if !reflect.DeepEqual(cic.Spec.Parameters, oic.Spec.Parameters) {
				klog.InfoS("update ingressClass related to the ingress controller", "ingressClass", klog.KObj(cic))
				err := c.ingressClassLister.Update(cic)
				if err != nil {
					klog.InfoS("error updating ingressClass in store", "ingressClass", klog.KObj(cic), "error", err)
					return
				}
			}
		},

		// ingressClass webhook filter ingressClass which is relevant to ingress
		DeleteFunc: func(obj interface{}) {
			ingressClass := obj.(*networkingv1.IngressClass)
			_, err := c.ingressClassLister.ByKey(ingressClass.Name)
			if err == nil {
				klog.InfoS("delete ingressClass related to the ingress controller", "ingressClass", klog.KObj(ingressClass))
				err = c.ingressClassLister.Delete(ingressClass)
				if err != nil {
					klog.InfoS("error removing ingressClass from store", "ingressClass", klog.KObj(ingressClass), "error", err)
					return
				}
			}
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
			ingresses := c.GetProjectIngresses(newAddProjects)
			for _, ingress := range ingresses {
				ic, err := c.GetIngressClass(ingress, icConfig)
				if err != nil {
					klog.InfoS("Ignoring ingress because of error while validating ingress class", "ingress", klog.KObj(ingress), "error", err)
					return
				}
				klog.InfoS("Found valid IngressClass", "ingress", klog.KObj(ingress), "ingressClass", ic)
				c.handleObject(ingress)
			}
		},
	})
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

	ingclasses, err := c.ingressClassAllLister.List(labels.Everything())
	if err != nil {
		klog.Error("error list all ingressClasses", err)
	} else {
		for _, ingcls := range ingclasses {
			if !CheckIngressClass(ingcls, c.icConfig) {
				continue
			}
			klog.InfoS("add legacy ingressClass related to the ingress controller", "ingressClass", klog.KObj(ingcls))
			err = c.ingressClassLister.Add(ingcls)
			if err != nil {
				klog.InfoS("error adding ingressClass to store", "ingressClass", klog.KObj(ingcls), "error", err)
				continue
			}
		}
	}

	ings, err := c.ingressLister.Ingresses("").List(labels.Everything())
	if err != nil {
		klog.Error("error list all ingresses", err)
	} else {
		// ensure legacy ingresses will be transformed
		for _, ing := range ings {
			if c.needEnqueueObject(ing, true) {
				klog.Infof("enqueue unprocessed ing: %s/%s %s", ing.Namespace, ing.Name, ing.ResourceVersion)
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
// objects' metadata.ownerReferences field for an appropriate OwnerReference.
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
	if c.needEnqueueObject(object, false) {
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
		// Finally, if no error occurs we Forget this item, so it does not
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
func (c *Controller) setFtDefault(ingress *networkingv1.Ingress, ft *m.Frontend) bool {
	if !isDefaultBackend(ingress) {
		return false
	}

	annotations := ingress.GetAnnotations()
	backendProtocol := strings.ToLower(annotations[ALBBackendProtocolAnnotation])
	defaultBackendService := ingress.Spec.DefaultBackend.Service
	portInService, err := c.KubernetesDriver.GetServicePortNumber(ingress.Namespace, defaultBackendService.Name, ToInStr(defaultBackendService.Port), corev1.ProtocolTCP)

	if err != nil {
		klog.Errorf("ingress setFtDefault: get port in service %s %s fail %v", err, ingress.Namespace, defaultBackendService.Name)
		return false
	}

	if ft.ServiceGroup == nil ||
		len(ft.ServiceGroup.Services) == 0 {
		ft.ServiceGroup = &alb2v1.ServiceGroup{
			Services: []alb2v1.Service{
				{
					Namespace: ingress.Namespace,
					Name:      defaultBackendService.Name,
					Port:      portInService,
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
		return true
	}

	// ft has default service
	originFtDefaultSvc := ft.ServiceGroup.Services[0]
	if !originFtDefaultSvc.Is(
		ingress.Namespace,
		ingress.Spec.DefaultBackend.Service.Name,
		portInService) {

		klog.Warningf("frontend %s already has default service %s/%s %v ,new default service %s/%s %v ingress %s, conflict",
			ft.Name,
			originFtDefaultSvc.Namespace,
			originFtDefaultSvc.Name,
			originFtDefaultSvc.Port,
			ingress.Namespace,
			ingress.Spec.DefaultBackend.Service.Name,
			portInService,
			ingress.Name,
		)
	}
	return false
}

// updateRule update or create rules for an ingress
func (c *Controller) updateRule(
	ingress *networkingv1.Ingress,
	ft *m.Frontend,
	host string,
	ingresPath networkingv1.HTTPIngressPath,
) error {
	ALBSSLAnnotation := fmt.Sprintf("alb.networking.%s/tls", config.Get("DOMAIN"))

	ingInfo := fmt.Sprintf("%s/%s", ingress.Namespace, ingress.Name)

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

	ruleAnnotation := ctl.GenerateRuleAnnotationFromIngressAnnotation(ingress.Name, annotations)

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

	ingressBackend := ingresPath.Backend.Service
	portInService, err := c.KubernetesDriver.GetServicePortNumber(ingress.Namespace, ingressBackend.Name, ToInStr(ingressBackend.Port), corev1.ProtocolTCP)
	if err != nil {
		return fmt.Errorf("get port in svc %s/%s %v fail err %v", ingress.Namespace, ingressBackend.Name, ingressBackend.Port, err)
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
			rule.CORSAllowHeaders == corsAllowHeaders &&
			rule.CORSAllowOrigin == corsAllowOrigin &&
			rule.BackendProtocol == backendProtocol &&
			rule.RedirectURL == redirectURL &&
			rule.RedirectCode == redirectCode &&
			rule.VHost == vhost {
			// already have

			// FIX: http://jira.alaudatech.com/browse/DEV-16951
			if rule.ServiceGroup != nil {
				found := false
				for _, svc := range rule.ServiceGroup.Services {
					if svc.Name == ingresPath.Backend.Service.Name {
						found = true
						break
					}
				}
				if !found {
					// when add a new service to service group we need to re calculate weight
					newsvc := alb2v1.Service{
						Namespace: ingress.Namespace,
						Name:      ingressBackend.Name,
						Port:      portInService,
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
				err = c.KubernetesDriver.UpdateRule(rule)
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
	pathType := networkingv1.PathTypeImplementationSpecific
	if ingresPath.PathType != nil {
		pathType = *ingresPath.PathType
	}

	ingVersion := ingress.ResourceVersion

	rule, err := ft.NewRule(ingInfo, host, url, rewriteTarget, backendProtocol, certs[host], enableCORS, corsAllowHeaders, corsAllowOrigin, redirectURL, redirectCode, vhost, DefaultPriority, pathType, ingVersion, ruleAnnotation)

	if err != nil {
		klog.Error(err)
		return err
	}
	rule.ServiceGroup = &alb2v1.ServiceGroup{
		Services: []alb2v1.Service{
			{
				Namespace: ingress.Namespace,
				Name:      ingresPath.Backend.Service.Name,
				Port:      portInService,
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
			"Create rule %+v for ingress %s %s failed: %s",
			*rule, ingInfo, ingVersion, err.Error(),
		)
		return err
	}
	klog.Infof("Create rule %s for ingress %s %s success", rule.Name, ingInfo, ingVersion)
	return nil
}

func (c *Controller) onIngressCreateOrUpdate(ingress *networkingv1.Ingress) error {
	klog.Infof("on ingress create or update, %s/%s %s", ingress.Namespace, ingress.Name, ingress.ResourceVersion)
	ALBSSLAnnotation := fmt.Sprintf("alb.networking.%s/tls", config.Get("DOMAIN"))
	IngressHTTPPort := config.GetInt("INGRESS_HTTP_PORT")
	IngressHTTPSPort := config.GetInt("INGRESS_HTTPS_PORT")

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

	// for default backend, we will not create rules but save services to frontends' service-group
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
		err = fmt.Errorf("port %d is not an HTTP port, protocol: %s", IngressHTTPPort, httpFt.Protocol)
		klog.Error(err)
		return err
	}
	if httpsFt != nil {
		if httpsFt.Protocol != m.ProtoHTTPS {
			err = fmt.Errorf("port %d is not an HTTPS port, protocol: %s", IngressHTTPSPort, httpsFt.Protocol)
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
			for _, proto := range []alb2v1.FtProtocol{m.ProtoHTTPS, m.ProtoHTTP} {
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
							"Update %s rule failed for ingress %s/%s %s with host=%s, path=%s",
							proto,
							ingress.Namespace,
							ingress.Name,
							ingress.ResourceVersion,
							host,
							p.Path,
						)
						return err
					} else {
						klog.Infof(
							"Update %s rule success for ingress %s/%s %s with host=%s, path=%s",
							proto,
							ingress.Namespace,
							ingress.Name,
							ingress.ResourceVersion,
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
	IngressHTTPPort := config.GetInt("INGRESS_HTTP_PORT")
	IngressHTTPSPort := config.GetInt("INGRESS_HTTPS_PORT")

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
				// wipe default backend
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

					klog.Infof("delete-rules ns:%s ingress name:%s  rule name %s reason: ingress-delete", namespace, name, rule.Name)
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

func (c *Controller) needEnqueueObject(object interface{}, needCheckIngressClass bool) (rv bool) {
	icConfig := c.icConfig
	reason := ""
	obj := object.(metav1.Object)
	logTag := fmt.Sprintf("sync-ingress needqueue ingress %s/%s rv %v", obj.GetNamespace(), obj.GetName(), obj.GetResourceVersion())
	log := func(format string, a ...interface{}) {
		klog.Infof(fmt.Sprintf("%s: %s", logTag, format), a...)
	}

	log("enter")
	IngressHTTPPort := config.GetInt("INGRESS_HTTP_PORT")
	IngressHTTPSPort := config.GetInt("INGRESS_HTTPS_PORT")

	defer func() {
		log("result %v reason %s", rv, reason)
	}()

	if needCheckIngressClass {
		ing := object.(*networkingv1.Ingress)
		_, err := c.GetIngressClass(ing, icConfig)
		if err != nil {
			klog.InfoS("Ignoring ingress because of error while validating ingress class", "ingress", klog.KObj(obj), "error", err)
			return false
		}
	}

	alb, err := c.KubernetesDriver.LoadALBbyName(config.Get("NAMESPACE"), config.Get("NAME"))
	if err != nil {
		msg := fmt.Sprintf("get alb res failed, %+v", err)
		klog.Errorf("%s: %s", logTag, msg)
		reason = msg
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
			reason = fmt.Sprintf("role port must have both http and https port, http %v %v, https %v %v", IngressHTTPPort, hasHTTPPort, IngressHTTPSPort, hasHTTPSPort)
			return false
		}
		if (funk.Contains(httpPortProjects, m.ProjectALL) || funk.Contains(httpPortProjects, belongProject)) &&
			(funk.Contains(httpsPortProjects, m.ProjectALL) || funk.Contains(httpsPortProjects, belongProject)) {
			return true
		}
		reason = fmt.Sprintf("role port belong project %v, not match http %v, https %v", belongProject, httpPortProjects, httpsPortProjects)
		return false
	}

	projects := ctl.GetOwnProjects(alb.Name, alb.Labels)
	if funk.Contains(projects, m.ProjectALL) {
		return true
	}
	if funk.Contains(projects, belongProject) {
		return true
	}
	reason = fmt.Sprintf("role instance,project %v belog %v", projects, belongProject)
	return false
}

func (c *Controller) GetProjectIngresses(projects []string) []*networkingv1.Ingress {
	if funk.ContainsString(projects, m.ProjectALL) {
		ingress, err := c.ingressLister.Ingresses("").List(labels.Everything())
		if err != nil {
			klog.Error(err)
			return nil
		}
		return ingress
	}
	var allIngresses []*networkingv1.Ingress
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
