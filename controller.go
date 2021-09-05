package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type IngressController struct {
	kubernetes.Interface
	interval      time.Duration
	staticAddress string
}

// NewIngressController initializes a new IngressController.
func NewIngressController(interval time.Duration, staticAddress string) (*IngressController, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	controller := &IngressController{
		Interface:     client,
		interval:      interval,
		staticAddress: staticAddress,
	}

	return controller, nil
}

func (i *IngressController) runOnce(ctx context.Context) error {
	ingresses, err := i.NetworkingV1().Ingresses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, ingress := range ingresses.Items {
		err = i.updateIngress(ctx, &ingress)
		if err != nil {
			log.Error(err)
			continue
		}
	}

	return nil
}

// Run runs the controller loop until it receives a stop signal over the stop
// channel.
func (i *IngressController) Run(ctx context.Context) {
	for {
		err := i.runOnce(ctx)
		if err != nil {
			log.Error(err)
		}

		select {
		case <-time.After(i.interval):
		case <-ctx.Done():
			log.Info("Terminating main controller loop.")
			return
		}
	}
}

// updateIngress finds all the pod backends for an ingress and sets the
// LoadBalancer IP field to the HostIP of the node with most pods serving the
// ingress.
func (i *IngressController) updateIngress(ctx context.Context, ingress *networkingv1.Ingress) error {
	host := i.staticAddress
	if host == "" {
		hosts := make(map[string]uint)
		for _, rule := range ingress.Spec.Rules {
			for _, path := range rule.HTTP.Paths {
				svc, err := i.CoreV1().Services(ingress.Namespace).Get(ctx, path.Backend.Service.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}

				opts := metav1.ListOptions{
					LabelSelector: labels.Set(svc.Spec.Selector).String(),
				}

				pods, err := i.CoreV1().Pods(svc.Namespace).List(ctx, opts)
				if err != nil {
					return err
				}

				for _, pod := range pods.Items {
					if pod.Status.Phase != v1.PodRunning {
						continue
					}

					// resolve external IP from node
					node, err := i.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
					if err != nil {
						return err
					}
					nodeAddress := ""
					for _, address := range node.Status.Addresses {
						if address.Type == v1.NodeExternalIP {
							nodeAddress = address.Address
							break
						}
					}

					hosts[nodeAddress] += 1
				}
			}
		}

		if len(hosts) == 0 {
			log.Info("No backends found for ingress, can't update ingress host field")
			return nil
		}

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
	}

	ingress.Status.LoadBalancer.Ingress = []v1.LoadBalancerIngress{{
		IP: host,
	}}

	_, err := i.NetworkingV1().Ingresses(ingress.Namespace).UpdateStatus(ctx, ingress, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"ingress": ingress.Name,
		"ip":      host,
	}).Info()

	return nil
}
