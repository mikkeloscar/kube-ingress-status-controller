FROM alpine:latest
MAINTAINER Mikkel Oscar Lyderik Larsen <m@moscar.net>

# add binary
ADD build/linux/kube-ingress-status-controller /

ENTRYPOINT ["/kube-ingress-status-controller"]
