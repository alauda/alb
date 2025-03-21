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
	cus "alauda.io/alb2/pkg/controller/extctl"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	networkinginformers "k8s.io/client-go/informers/networking/v1"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
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
	ingressLister networkinglisters.IngressLister
	ruleLister    listerv1.RuleLister

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

	albInformer          informerv2.ALB2Informer
	ingressInformer      networkinginformers.IngressInformer
	ingressClassInformer networkinginformers.IngressClassInformer
	log                  logr.Logger
	IngressSelect
	*config.Config
	cus cus.ExtCtl
}

// NewController returns a new sample controller
func NewController(d *driver.KubernetesDriver, informers driver.Informers, albCfg *config.Config, log logr.Logger) *Controller {
	alb2Informer := informers.Alb.Alb
	ruleInformer := informers.Alb.Rule
	ingressInformer := informers.K8s.Ingress
	ingressClassInformer := informers.K8s.IngressClass
	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	log.Info("Creating event broadcaster")
	eventLog := log.WithName("event")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(func(fmts string, args ...interface{}) {
		msg := fmt.Sprintf(fmts, args...)
		eventLog.Info(msg)
	})
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{Interface: d.Client.CoreV1().Events("")},
	)
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		corev1.EventSource{Component: fmt.Sprintf("alb2-%s", albCfg.GetAlbName()), Host: hostname},
	)

	controller := &Controller{
		ingressLister:        ingressInformer.Lister(),
		ruleLister:           ruleInformer.Lister(),
		workqueue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Ingresses"),
		recorder:             recorder,
		kd:                   NewDriver(d, albCfg, log.WithName("driver")),
		albInformer:          alb2Informer,
		ingressInformer:      ingressInformer,
		ingressClassInformer: ingressClassInformer,
		log:                  log,
		Config:               albCfg,
		IngressSelect: IngressSelect{
			cfg: Cfg2IngressSelectOpt(albCfg),
			drv: d,
		},
		cus: cus.NewExtensionCtl(cus.ExtCtlCfgOpt{
			Log:    log,
			Domain: albCfg.Domain,
		}),
	}
	return controller
}

func (c *Controller) setUpEventHandler() error {
	// 1: reconcile ingress when ingress create/update/delete
	_, err := c.ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			newIngress := obj.(*networkingv1.Ingress)
			c.log.Info(fmt.Sprintf("receive ingress %s/%s %s create event", newIngress.Namespace, newIngress.Name, newIngress.ResourceVersion))
			if !c.CheckShouldHandleViaIngressClass(newIngress) {
				c.log.Info("not our ingress ignore", "ing", newIngress)
				return
			}
			c.enqueue(IngKey(newIngress))
		},
		UpdateFunc: func(old, latest interface{}) {
			newIngress := latest.(*networkingv1.Ingress)
			oldIngress := old.(*networkingv1.Ingress)
			if newIngress.ResourceVersion == oldIngress.ResourceVersion {
				return
			}
			if reflect.DeepEqual(newIngress.Annotations, oldIngress.Annotations) && reflect.DeepEqual(newIngress.Spec, oldIngress.Spec) && reflect.DeepEqual(newIngress.Labels, oldIngress.Labels) {
				return
			}
			c.log.Info(fmt.Sprintf("receive ingress %s/%s update event  version:%s/%s", newIngress.Namespace, newIngress.Name, oldIngress.ResourceVersion, newIngress.ResourceVersion))
			// 如果更新成了别的ingressclass，我们也要去处理下，要cleanup
			if !c.CheckShouldHandleViaIngressClass(oldIngress) && !c.CheckShouldHandleViaIngressClass(newIngress) {
				c.log.Info("not our ingressclass ignore", "old-ing", oldIngress, "new-ing", newIngress)
				return
			}

			if c.CheckShouldHandleViaIngressClass(oldIngress) && !c.CheckShouldHandleViaIngressClass(newIngress) {
				c.log.Info("change to other ingress class", "old-ing", oldIngress, "new-ing", newIngress)
				if err := c.onIngressclassChange(newIngress); err != nil {
					c.log.Error(err, "handle ingressclass change fail")
				}
			}

			c.enqueue(IngKey(newIngress))
		},
		DeleteFunc: func(obj interface{}) {
			ingress := obj.(*networkingv1.Ingress)
			c.log.Info(fmt.Sprintf("receive ingress %s/%s %s delete event", ingress.Namespace, ingress.Name, ingress.ResourceVersion))
			if !c.CheckShouldHandleViaIngressClass(ingress) {
				c.log.Info("not our ingressclass ignore", "ing", ingress)
				return
			}
			c.enqueue(IngKey(ingress))
		},
	})
	if err != nil {
		return err
	}

	// 3. reconcile ingress when alb project change.
	// 4. reconcile ingress when alb address change.
	_, err = c.albInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, latest interface{}) {
			newAlb2 := latest.(*alb2v2.ALB2)
			oldAlb2 := old.(*alb2v2.ALB2)
			if !newAlb2.GetDeletionTimestamp().IsZero() {
				c.onAlbDelete(newAlb2)
				return
			}

			if oldAlb2.ResourceVersion == newAlb2.ResourceVersion {
				return
			}
			if reflect.DeepEqual(oldAlb2.Labels, newAlb2.Labels) && reflect.DeepEqual(oldAlb2.Spec, newAlb2.Spec) {
				return
			}
			ns, name := c.GetAlbNsAndName()
			if newAlb2.Name != name || newAlb2.Namespace != ns {
				return
			}
			c.log.Info("find alb changed", "diff", cmp.Diff(oldAlb2, newAlb2))
			err = c.onAlbChangeUpdateIngressStatus(oldAlb2, newAlb2)
			if err != nil {
				c.log.Error(err, "update ingress status fail")
			}
			err = c.SyncAll()
			if err != nil {
				c.log.Error(err, "reprocess all ingress when alb changed failed")
			}
		},
	})
	if err != nil {
		return err
	}
	// watch ns change it may add or remove project label which alb care
	_, err = c.kd.Informers.K8s.Namespace.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, latest interface{}) {
			newNs := latest.(*corev1.Namespace)
			oldNs := old.(*corev1.Namespace)
			if oldNs.ResourceVersion == newNs.ResourceVersion {
				return
			}
			if reflect.DeepEqual(oldNs.Labels, newNs.Labels) {
				return
			}
			oldNsProject := mapset.NewSet(ctl.GetOwnProjectsFromLabel(oldNs.Name, oldNs.Labels, c.Domain)...)
			newNsProject := mapset.NewSet(ctl.GetOwnProjectsFromLabel(newNs.Name, newNs.Labels, c.Domain)...)
			if oldNsProject.Equal(newNsProject) {
				return
			}
			alb, err := c.kd.LoadALBbyName(c.Ns, c.Name)
			if err != nil {
				c.log.Error(err, "reprocess ns ingress when ns changed failed")
			}
			albProject := mapset.NewSet(ctl.GetOwnProjectsFromAlb(alb.Alb)...)
			if len((oldNsProject.Union(newNsProject)).Intersect(albProject).ToSlice()) == 0 {
				return
			}
			err = c.syncNs(newNs.Name)
			if err != nil {
				c.log.Error(err, "reprocess all ingress when ns changed failed")
			}
		},
	})
	if err != nil {
		return err
	}

	return nil
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
	err := c.setUpEventHandler()
	if err != nil {
		return err
	}
	// Start the informer factories to begin populating the informer caches
	c.log.Info("start ingress controller")
	c.log.Info("Starting workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.RunWorker, time.Second, stopCh)
	}

	c.log.Info("Started workers")

	// init sync
	err = c.SyncAll()
	if err != nil {
		c.log.Error(err, "init sync fail")
	}
	<-stopCh
	c.log.Info("Shutting down workers")

	return nil
}

func (c *Controller) syncNs(ns string) error {
	ings, err := c.kd.ListIngressInNs(ns)
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

// sync in startup
func (c *Controller) SyncAll() error {
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

func (c *Controller) DrainAndStop() error {
	c.log.Info("DrainAndStop", "workqueue", c.workqueue.Len())
	c.workqueue.ShutDownWithDrain()
	return nil
}

func (c *Controller) GetWorkqueueLen() int {
	return c.workqueue.Len()
}

// RunWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) RunWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the Reconcile.
// TODO use controller-runtime
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	func(obj interface{}) {
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
			c.log.Info("invalid workqueue key type", "obj", obj)
		}
		// Run the Reconcile, passing it the namespace/name string of the
		// Foo resource to be synced.
		if reque, err := c.Reconcile(key); err != nil || reque {
			// Put the item back on the workqueue to handle any transient errors.
			c.log.Info("requeue", "ing", key, "err", err, "requeue", reque)
			c.workqueue.AddRateLimited(key)
		}
		// Finally, if no error occurs we Forget this item, so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
	}(obj)

	return true
}
