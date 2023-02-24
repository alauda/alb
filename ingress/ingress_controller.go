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
	"time"

	"alauda.io/alb2/config"
	ctl "alauda.io/alb2/controller"
	"alauda.io/alb2/driver"
	alb2v2 "alauda.io/alb2/pkg/apis/alauda/v2beta1"
	informerv2 "alauda.io/alb2/pkg/client/informers/externalversions/alauda/v2beta1"
	listerv1 "alauda.io/alb2/pkg/client/listers/alauda/v1"
	"github.com/go-logr/logr"
	"github.com/thoas/go-funk"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func Key(obj metav1.Object) string {
	if obj == nil {
		return ""
	}
	if val := reflect.ValueOf(obj); val.Kind() == reflect.Ptr && val.IsNil() {
		return ""
	}

	return fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
}

func (c *Controller) StartIngressLoop(ctx context.Context) error {
	return c.Run(1, ctx)
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

	kd *IngressDriver

	// configuration for ingressClass
	icConfig *config.IngressClassConfiguration

	albInformer          informerv2.ALB2Informer
	ingressInformer      networkinginformers.IngressInformer
	ingressClassInformer networkinginformers.IngressClassInformer
	log                  logr.Logger
	config.IConfig
}

// NewController returns a new sample controller
func NewController(d *driver.KubernetesDriver, informers driver.Informers, albCfg config.IConfig, log logr.Logger) *Controller {

	alb2Informer := informers.Alb.Alb
	ruleInformer := informers.Alb.Rule
	ingressInformer := informers.K8s.Ingress
	ingressClassInformer := informers.K8s.IngressClass
	namespaceLister := informers.K8s.Namespace.Lister()
	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	log.Info("Creating event broadcaster")
	eventlog := log.WithName("event")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(func(fmts string, args ...interface{}) {
		msg := fmt.Sprintf(fmts, args...)
		eventlog.Info(msg)
	})
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{Interface: d.Client.CoreV1().Events("")},
	)
	hostname, _ := os.Hostname()
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		corev1.EventSource{Component: fmt.Sprintf("alb2-%s", config.Get("NAME")), Host: hostname},
	)

	domain := albCfg.GetDomain()
	ControllerName := fmt.Sprintf("%s/%s", domain, config.DefaultControllerName)

	controller := &Controller{
		ingressLister:         ingressInformer.Lister(),
		ingressClassAllLister: ingressClassInformer.Lister(),
		ingressClassLister:    ingressClassLister{cache.NewStore(cache.MetaNamespaceKeyFunc)},
		ruleLister:            ruleInformer.Lister(),
		namespaceLister:       namespaceLister,
		workqueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Ingresses"),
		recorder:              recorder,
		kd:                    NewDriver(d, albCfg, log.WithName("driver")),
		icConfig:              &config.IngressClassConfiguration{Controller: ControllerName, AnnotationValue: config.DefaultControllerName, WatchWithoutClass: true, IgnoreIngressClass: false, IngressClassByName: true},
		albInformer:           alb2Informer,
		ingressInformer:       ingressInformer,
		ingressClassInformer:  ingressClassInformer,
		log:                   log,
		IConfig:               albCfg,
	}
	return controller
}

func (c *Controller) setUpEventHandler() {

	// icConfig: configuration items for ingressClass
	icConfig := c.icConfig
	// 1: reconcile ingress when ingress create/update/delete
	c.ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			newIngress := obj.(*networkingv1.Ingress)
			c.log.Info(fmt.Sprintf("receive ingress %s/%s %s create event", newIngress.Namespace, newIngress.Name, newIngress.ResourceVersion))
			ic, err := c.GetIngressClass(newIngress, icConfig)
			if err != nil {
				c.log.Info("Ignoring ingress because of error while validating ingress class", "ingress", Key(newIngress), "error", err)
				return
			}
			c.log.Info("Found valid IngressClass", "ingress", Key(newIngress), "ingressClass", ic)
			c.enqueue(IngKey(newIngress))
		},
		UpdateFunc: func(old, new interface{}) {
			var errOld, errCur error
			var classCur string
			newIngress := new.(*networkingv1.Ingress)
			oldIngress := old.(*networkingv1.Ingress)
			if newIngress.ResourceVersion == oldIngress.ResourceVersion {
				return
			}
			if reflect.DeepEqual(newIngress.Annotations, oldIngress.Annotations) && reflect.DeepEqual(newIngress.Spec, oldIngress.Spec) {
				return
			}
			_, errOld = c.GetIngressClass(oldIngress, icConfig)
			classCur, errCur = c.GetIngressClass(newIngress, icConfig)

			c.log.Info(fmt.Sprintf("receive ingress %s/%s update event  version:%s/%s", newIngress.Namespace, newIngress.Name, oldIngress.ResourceVersion, newIngress.ResourceVersion))

			if errOld != nil && errCur == nil {
				c.log.Info("creating ingress", "ingress", Key(newIngress), "ingressClass", classCur)
			} else if errOld == nil && errCur != nil {
				c.log.Info("removing ingress because of ingressClass changed to other ingress controller", "ingress", Key(newIngress))
				c.enqueue(IngKey(oldIngress))
				return
			} else if errCur == nil && !reflect.DeepEqual(old, new) {
				c.log.Info("update ingress", "ingress", Key(newIngress), "ingressClass", classCur)
			} else {
				return
			}
			c.enqueue(IngKey(newIngress))
		},
		DeleteFunc: func(obj interface{}) {
			ingress := obj.(*networkingv1.Ingress)
			c.log.Info(fmt.Sprintf("receive ingress %s/%s %s delete event", ingress.Namespace, ingress.Name, ingress.ResourceVersion))
			_, err := c.GetIngressClass(ingress, icConfig)
			if err != nil {
				c.log.Info("Ignoring ingress because of error while validating ingress class", "ingress", Key(ingress), "error", err)
				return
			}
			c.enqueue(IngKey(ingress))
		},
	})

	// TODO remove this
	c.ingressClassInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ingressClass := obj.(*networkingv1.IngressClass)
			if !CheckIngressClass(ingressClass, icConfig) {
				return
			}
			c.log.Info("add new ingressClass related to the ingress controller", "ingressClass", Key(ingressClass))
			err := c.ingressClassLister.Add(ingressClass)
			if err != nil {
				c.log.Info("error adding ingressClass to store", "ingressClass", Key(ingressClass), "error", err)
				return
			}
		},

		UpdateFunc: func(old, cur interface{}) {
			oic := old.(*networkingv1.IngressClass)
			cic := cur.(*networkingv1.IngressClass)
			if cic.Spec.Controller != icConfig.Controller {
				c.log.Info("ignoring ingressClass as the spec.controller is not the same of this ingress", "ingressClass", Key(cic))
				return
			}
			if !reflect.DeepEqual(cic.Spec.Parameters, oic.Spec.Parameters) {
				c.log.Info("update ingressClass related to the ingress controller", "ingressClass", Key(cic))
				err := c.ingressClassLister.Update(cic)
				if err != nil {
					c.log.Info("error updating ingressClass in store", "ingressClass", Key(cic), "error", err)
					return
				}
			}
		},

		// ingressClass webhook filter ingressClass which is relevant to ingress
		DeleteFunc: func(obj interface{}) {
			ingressClass := obj.(*networkingv1.IngressClass)
			_, err := c.ingressClassLister.ByKey(ingressClass.Name)
			if err == nil {
				c.log.Info("delete ingressClass related to the ingress controller", "ingressClass", Key(ingressClass))
				err = c.ingressClassLister.Delete(ingressClass)
				if err != nil {
					c.log.Info("error removing ingressClass from store", "ingressClass", Key(ingressClass), "error", err)
					return
				}
			}
		},
	})
	// 3. reconcile ingress when alb project change.
	c.albInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newAlb2 := new.(*alb2v2.ALB2)
			oldAlb2 := old.(*alb2v2.ALB2)
			if oldAlb2.ResourceVersion == newAlb2.ResourceVersion {
				return
			}
			if reflect.DeepEqual(oldAlb2.Labels, newAlb2.Labels) {
				return
			}
			newProjects := ctl.GetOwnProjectsFromAlb(newAlb2.Name, newAlb2.Labels, &newAlb2.Spec)
			oldProjects := ctl.GetOwnProjectsFromAlb(oldAlb2.Name, oldAlb2.Labels, &oldAlb2.Spec)
			newAddProjects := funk.SubtractString(newProjects, oldProjects)
			c.log.Info(fmt.Sprintf("own projects: %v, new add projects: %v", newProjects, newAddProjects))
			ingresses := c.GetProjectIngresses(newAddProjects)
			for _, ingress := range ingresses {
				ic, err := c.GetIngressClass(ingress, icConfig)
				if err != nil {
					c.log.Info("Ignoring ingress because of error while validating ingress class", "ingress", Key(ingress), "error", err)
					return
				}
				c.log.Info("Found valid IngressClass", "ingress", Key(ingress), "ingressClass", ic)
				c.enqueue(IngKey(ingress))
			}
		},
	})
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, ctx context.Context) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()
	stopCh := ctx.Done()

	c.log.Info("Setting up event handlers")
	c.setUpEventHandler()
	// Start the informer factories to begin populating the informer caches
	c.log.Info("start ingress controller")
	c.log.Info("Starting workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	c.log.Info("Started workers")

	ingclasses, err := c.ingressClassAllLister.List(labels.Everything())
	if err != nil {
		c.log.Error(err, "error list all ingressClasses")
	} else {
		for _, ingcls := range ingclasses {
			if !CheckIngressClass(ingcls, c.icConfig) {
				continue
			}
			c.log.Info("add legacy ingressClass related to the ingress controller", "ingressClass", Key(ingcls))
			err = c.ingressClassLister.Add(ingcls)
			if err != nil {
				c.log.Info("error adding ingressClass to store", "ingressClass", Key(ingcls), "error", err)
				continue
			}
		}
	}

	// init sync
	err = c.initSync()
	if err != nil {
		c.log.Error(err, "init sync fail")
	}
	<-stopCh
	c.log.Info("Shutting down workers")

	return nil
}

// sync in startup
func (c *Controller) initSync() error {
	ings, err := c.kd.ListAllIngress()
	if err != nil {
		return err
	}
	// ensure legacy ingresses will be transformed
	for _, ing := range ings {
		c.log.Info(fmt.Sprintf("enqueue unprocessed ing: %s/%s %s", ing.Namespace, ing.Name, ing.ResourceVersion))
		c.enqueue(IngKey(ing))
	}
	return nil
}

func (c *Controller) enqueue(key client.ObjectKey) {
	c.workqueue.AddRateLimited(key)
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the Reconcile.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	_ = func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key client.ObjectKey
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(client.ObjectKey); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			c.log.Info("invalud workerq key type", "obj", obj)
			return nil
		}
		// Run the Reconcile, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.Reconcile(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return nil
		}
		// Finally, if no error occurs we Forget this item, so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		return nil
	}(obj)

	return true
}
