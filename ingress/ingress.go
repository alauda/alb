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
	"time"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	extsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	extsinformers "k8s.io/client-go/informers/extensions/v1beta1"
	"k8s.io/client-go/kubernetes"
	scheme "k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	extslisters "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	"alb2/config"
	"alb2/driver"
	m "alb2/modules"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Ingress is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	// MessageResourceSynced is the message used for an Event fired when a Ingress
	// is synced successfully
	MessageResourceSynced = "Ingress synced successfully"
)

// MainLoop is the entrypoint of this controller
func MainLoop(ctx context.Context) {
	drv, err := driver.GetDriver()
	if err != nil {
		panic(err)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(drv.Client, time.Second*180)
	controller := NewController(
		drv.Client,
		kubeInformerFactory.Extensions().V1beta1().Ingresses(),
	)

	kubeInformerFactory.Start(ctx.Done())

	if err = controller.Run(1, ctx.Done()); err != nil {
		glog.Errorf("Error running controller: %s", err.Error())
	}
}

// Controller is the controller implementation for Foo resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface

	ingressLister extslisters.IngressLister
	ingressSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	kubeclientset kubernetes.Interface,
	ingressInformer extsinformers.IngressInformer) *Controller {

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	glog.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(
		&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")},
	)
	recorder := eventBroadcaster.NewRecorder(
		scheme.Scheme,
		corev1.EventSource{Component: "alb2", Host: config.Get("NAME")},
	)

	controller := &Controller{
		kubeclientset: kubeclientset,
		ingressLister: ingressInformer.Lister(),
		ingressSynced: ingressInformer.Informer().HasSynced,
		workqueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.DefaultControllerRateLimiter(),
			"Ingresses",
		),
		recorder: recorder,
	}

	glog.Info("Setting up event handlers")

	ingressInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newIngress := new.(*extsv1beta1.Ingress)
			oldIngress := old.(*extsv1beta1.Ingress)
			if newIngress.ResourceVersion == oldIngress.ResourceVersion {
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
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
	glog.Info("start ingress controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.ingressSynced); !ok {
		err := fmt.Errorf("failed to wait for caches to sync")
		glog.Error(err)
		return err
	}

	glog.Info("Starting workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

	return nil
}

func (c *Controller) enqueue(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Foo resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Foo resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			err := fmt.Errorf("error decoding object, invalid type")
			glog.Error(err)
			runtime.HandleError(err)
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		glog.Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.Infof("Processing object: %s", object.GetName())
	annotations := object.GetAnnotations()
	if annotations["kubernetes.io/ingress.clas"] == "" ||
		annotations["kubernetes.io/ingress.clas"] == config.Get("NAME") {
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
		glog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		glog.Error(err)
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
	glog.Infof("Sync ingress %s/%s", namespace, name)
	ingress, err := c.ingressLister.Ingresses(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return c.onIngressDelete(name, namespace)
		}
		glog.Errorf("Handle %s.%s failed: %s", name, namespace, err.Error())
		return err
	}
	glog.Infof("Process ingress %s.%s", ingress.Name, ingress.Namespace)
	err = c.onIngressCreateOrUpdate(ingress)
	if err != nil {
		return err
	}
	c.recorder.Event(ingress, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)

	return nil
}

func (c *Controller) setFtDefault(ingress *extsv1beta1.Ingress, ft *m.Frontend) bool {
	if ingress.Spec.Backend == nil {
		return false
	}
	needSave := false
	if ft.ServiceGroup == nil ||
		len(ft.ServiceGroup.Services) == 0 {
		ft.ServiceGroup = &m.ServicceGroup{
			Services: []m.Service{
				m.Service{
					Namespace: ingress.Namespace,
					Name:      ingress.Spec.Backend.ServiceName,
					Port:      int(ingress.Spec.Backend.ServicePort.IntVal),
				},
			},
		}
		ft.Source = &m.SourceInfo{
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

			//TODO Add event here
		}
	}
	return needSave
}

func (c *Controller) updateRule(
	ingress *extsv1beta1.Ingress,
	ft *m.Frontend,
	host string,
	ingresPath extsv1beta1.HTTPIngressPath,
) error {
	url := ingresPath.Path
	for _, rule := range ft.Rules {
		if rule.Source == nil {
			// user defined rule
			continue
		}
		if rule.Source.Type == m.TypeIngress &&
			rule.Source.Name == ingress.Name &&
			rule.Source.Namespace == ingress.Namespace &&
			rule.Domain == host &&
			rule.URL == url {
			// already have
			return nil
		}
	}
	rule, err := ft.NewRule(host, url, "")
	if err != nil {
		glog.Error(err)
		return err
	}
	rule.ServiceGroup = &m.ServicceGroup{
		Services: []m.Service{
			m.Service{
				Namespace: ingress.Namespace,
				Name:      ingresPath.Backend.ServiceName,
				Port:      int(ingresPath.Backend.ServicePort.IntVal),
			},
		},
	}
	rule.Source = &m.SourceInfo{
		Type:      m.TypeIngress,
		Namespace: ingress.Namespace,
		Name:      ingress.Name,
	}
	err = driver.CreateRule(rule)
	if err != nil {
		glog.Errorf(
			"Create rule %+v for ingress %s.%s failed: %s",
			*rule, ingress.Namespace, ingress.Name, err.Error(),
		)
		return err
	}
	return nil
}

func (c *Controller) onIngressCreateOrUpdate(ingress *extsv1beta1.Ingress) error {
	alb, err := driver.LoadALBbyName(
		config.Get("NAMESPACE"),
		config.Get("NAME"),
	)
	if err != nil {
		glog.Error(err)
		return err
	}
	var ft *m.Frontend
	for _, ft = range alb.Frontends {
		if ft.Port == 80 {
			break
		}
	}
	if ft != nil && ft.Port != 80 && ft.Protocol != m.ProtoHTTP {
		err = fmt.Errorf("Port 80 is not an HTTP port")
		glog.Error(err)
		return err
	}

	newFrontend := false
	if ft == nil {
		ft, err = alb.NewFrontend(80, m.ProtoHTTP)
		if err != nil {
			glog.Error(err)
			return err
		}
		newFrontend = true
	}
	needSave := c.setFtDefault(ingress, ft)
	if newFrontend || needSave {
		err = driver.UpsertFrontends(alb, ft)
		if err != nil {
			glog.Errorf("upsert ft failed: %s", err)
			return err
		}
	}

	for _, r := range ingress.Spec.Rules {
		host := r.Host
		httpRules := r.IngressRuleValue.HTTP
		if httpRules == nil {
			glog.Infof("No http rule found on ingress %s/%s under host %s.",
				ingress.Namespace,
				ingress.Name,
				host,
			)
			continue
		}
		for _, p := range httpRules.Paths {
			err = c.updateRule(ingress, ft, host, p)
			if err != nil {
				glog.Errorf(
					"Update rule failed for ingress %s/%s with host=%s, path=%s",
					ingress.Namespace,
					ingress.Name,
					host,
					p.Path,
				)
			}
		}
	}
	return nil
}

func (c *Controller) onIngressDelete(name, namespace string) error {
	alb, err := driver.LoadALBbyName(
		config.Get("NAMESPACE"),
		config.Get("NAME"),
	)
	if err != nil {
		glog.Error(err)
		return err
	}
	var ft *m.Frontend
	for _, ft = range alb.Frontends {
		if ft.Port == 80 {
			break
		}
	}
	if ft == nil || ft.Port != 80 {
		// No frontend found
		return nil
	}
	if ft.Source != nil &&
		ft.Source.Type == m.TypeIngress &&
		ft.Source.Namespace == namespace &&
		ft.Source.Name == name {
		ft.ServiceGroup = nil
		ft.Source = nil
		err = driver.UpsertFrontends(alb, ft)
		if err != nil {
			glog.Errorf("upsert ft failed: %s", err)
			return err
		}
	}

	for _, rule := range ft.Rules {
		if rule.Source != nil &&
			rule.Source.Type == m.TypeIngress &&
			rule.Source.Namespace == namespace &&
			rule.Source.Name == name {

			err = driver.DeleteRule(rule)
			if err != nil {
				glog.Errorf("upsert ft failed: %s", err)
				return err
			}
		}
	}
	return nil
}
