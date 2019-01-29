package main

import (
	"fmt"
	"github.com/appscode/go/runtime"
	"github.com/appscode/go/wait"
	controllerv1alpha1 "github.com/kamolhasan/CRD-Controller/pkg/apis/crdcontroller.com/v1alpha1"
	clientset "github.com/kamolhasan/CRD-Controller/pkg/client/clientset/versioned"
	informers "github.com/kamolhasan/CRD-Controller/pkg/client/informers/externalversions/crdcontroller.com/v1alpha1"
	lister "github.com/kamolhasan/CRD-Controller/pkg/client/listers/crdcontroller.com/v1alpha1"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsinformer "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appslister "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"log"
	"time"
)

const controllerAgentName = "CRD-Controller"

const (
	SuccessSynced         = "Synced"
	ErrResourceExists     = "ErrResourceExists"
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	MessageResourceSynced = "Foo synced successfully"
)

// Controller is the controller implementation for Foo resources
type Controller struct {
	kubeclientset   kubernetes.Interface
	sampleclientset clientset.Interface

	deploymentsLister appslister.DeploymentLister
	deploymentsSynced cache.InformerSynced
	foosLister        lister.FooLister
	foosSynced        cache.InformerSynced

	workQueue workqueue.RateLimitingInterface
}

// NewController returns a new sample controller

func NewController(kubeclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	deploymentInformer appsinformer.DeploymentInformer,
	fooInformer informers.FooInformer) *Controller {

	controller := &Controller{
		kubeclientset:     kubeclientset,
		sampleclientset:   sampleclientset,
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		deploymentsLister: deploymentInformer.Lister(),
		foosLister:        fooInformer.Lister(),
		foosSynced:        fooInformer.Informer().HasSynced,
		workQueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Foos"),
	}

	log.Println("setting up event handlers")

	fooInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueFoo,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueFoo(new)
		},
	})
	return controller
}

func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workQueue.ShutDown()

	log.Println("Starting Foo Controller")

	log.Println("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.foosSynced); !ok {
		return fmt.Errorf("failed to wait for cache to sync")

	}
	log.Println("Starting workers")

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	log.Println("Worker started")
	<-stopCh
	log.Println("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.ProcessNextItem() {

	}

}

func (c *Controller) ProcessNextItem() bool {
	obj, shutdown := c.workQueue.Get()

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.workQueue.Done(obj)
		var key string
		var ok bool
		if key, ok = obj.(string); ok {
			c.workQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue b ut got %#v", obj))
			return nil

		}
		if err := c.syncHandler(key); err != nil {
			c.workQueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}

		c.workQueue.Forget(obj)
		log.Printf("successfully synced '%s'\n", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}
	return true
}

func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)

	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
	}

	foo, err := c.foosLister.Foos(namespace).Get(name)

	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("foo '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}
	deploymentName := foo.Spec.DeploymentName
	if deploymentName == "" {
		runtime.HandleError(fmt.Errorf("%s : deployment name must be specified", key))
		return nil
	}

	deployment, err := c.deploymentsLister.Deployments(namespace).Get(deploymentName)

	if errors.IsNotFound(err) {
		deployment, err = c.kubeclientset.AppsV1().Deployments(foo.Namespace).Create(newDeployment(foo))
	}
	if err != nil {
		return err
	}

	if foo.Spec.Replicas != nil && *foo.Spec.Replicas != *deployment.Spec.Replicas {
		log.Printf("Foo %s replicas: %d, deployment replicas: %d\n", name, *foo.Spec.Replicas, *deployment.Spec.Replicas)

		deployment, err = c.kubeclientset.AppsV1().Deployments(namespace).Update(newDeployment(foo))

		if err != nil {
			return err
		}
	}

	err = c.updateFooStatus(foo, deployment)
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) enqueueFoo(obj interface{}) {
	log.Println("Enqueueing Foo. . . ")
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workQueue.AddRateLimited(key)
}

func (c *Controller) updateFooStatus(foo *controllerv1alpha1.Foo, deployment *appv1.Deployment) error {

	fooCopy := foo.DeepCopy()
	fooCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas

	_, err := c.sampleclientset.CrdcontrollerV1alpha1().Foos(foo.Namespace).Update(fooCopy)

	return err

}

func newDeployment(foo *controllerv1alpha1.Foo) *appv1.Deployment {

	return &appv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      foo.Spec.DeploymentName,
			Namespace: foo.Namespace,
		},
		Spec: appv1.DeploymentSpec{
			Replicas: foo.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":        "nginx",
					"controller": foo.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":        "nginx",
						"controller": foo.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}
}
