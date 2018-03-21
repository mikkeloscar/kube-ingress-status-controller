package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"

	log "github.com/Sirupsen/logrus"
)

const (
	defaultInterval = "15s"
)

var (
	config struct {
		Interval       time.Duration
		IngressAddress string
	}
)

func init() {
	kingpin.Flag("interval", "Interval between checks.").
		Default(defaultInterval).DurationVar(&config.Interval)
	kingpin.Flag("ingress-ip-address", "Static IP address to be put on all ingresses.").
		StringVar(&config.IngressAddress)
}

func main() {
	kingpin.Parse()

	controller, err := NewIngressController(config.Interval, config.IngressAddress)
	if err != nil {
		log.Fatal(err)
	}

	stopChan := make(chan struct{}, 1)
	go handleSigterm(stopChan)

	controller.Run(stopChan)
}

func handleSigterm(stopChan chan struct{}) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	<-signals
	log.Info("Received Term signal. Terminating...")
	close(stopChan)
}
