package main

import (
	"time"

	log "github.com/Sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/rest"
)

type IngressController struct {
	kubernetes.Interface
	interval time.Duration
}

// NewIngressController initializes a new IngressController.
func NewIngressController(interval time.Duration) (*IngressController, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	controller := &IngressController{
		Interface: client,
		interval:  interval,
	}

	return controller, nil
}

func (i *IngressController) runOnce() error {
	ingresses, err := i.ExtensionsV1beta1().Ingresses(v1.NamespaceAll).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, ingress := range ingresses.Items {
		err = i.updateIngress(&ingress)
		if err != nil {
			log.Error(err)
			continue
		}
	}

	return nil
}

// Run runs the controller loop until it receives a stop signal over the stop
// channel.
func (i *IngressController) Run(stopChan <-chan struct{}) {
	for {
		err := i.runOnce()
		if err != nil {
			log.Error(err)
		}

		select {
		case <-time.After(i.interval):
		case <-stopChan:
			log.Info("Terminating main controller loop.")
			return
		}
	}
}

// updateIngress finds all the pod backends for an ingress and sets the
// LoadBalancer IP field to the HostIP of the node with most pods serving the
// ingress.
func (i *IngressController) updateIngress(ingress *v1beta1.Ingress) error {
	hosts := make(map[string]uint)
	for _, rule := range ingress.Spec.Rules {
		for _, path := range rule.HTTP.Paths {
			svc, err := i.CoreV1().Services(ingress.Namespace).Get(path.Backend.ServiceName, metav1.GetOptions{})
			if err != nil {
				return err
			}

			opts := metav1.ListOptions{
				LabelSelector: labels.Set(svc.Spec.Selector).String(),
			}

			pods, err := i.CoreV1().Pods(svc.Namespace).List(opts)
			if err != nil {
				return err
			}

			for _, pod := range pods.Items {
				if _, ok := hosts[pod.Status.HostIP]; ok {
					hosts[pod.Status.HostIP] += 1
				} else {
					hosts[pod.Status.HostIP] = 1
				}
			}
		}
	}

	var host string
	var max uint
	for ip, count := range hosts {
		log.WithFields(log.Fields{
			"host": ip,
			"num":  count,
		}).Debug()
		if count > max {
			host = ip
			max = count
		}
	}

	ingress.Status.LoadBalancer.Ingress = []v1.LoadBalancerIngress{{
		IP: host,
	}}

	_, err := i.ExtensionsV1beta1().Ingresses(ingress.Namespace).UpdateStatus(ingress)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"ingress": ingress.Name,
		"ip":      host,
	}).Info()

	return nil
}
