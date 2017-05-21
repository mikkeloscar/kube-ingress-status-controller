# Kube Ingress Status Controller
[![Build Status](https://travis-ci.org/mikkeloscar/kube-ingress-status-controller.svg?branch=master)](https://travis-ci.org/mikkeloscar/kube-ingress-status-controller)

Simple controller to set ingress status field (IP or hostname) based on Pod
distribution. E.g. if an ingress points to a service backend where most
targeted pods are on node X then the IP of node X will be set on the ingress
status field.
