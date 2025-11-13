FROM ubuntu:25.10 AS build

COPY . /blipblop/src/
WORKDIR /blipblop/src

ENV PATH="$PATH:/usr/local/go/bin"

ARG TARGETARCH

RUN set -x \
&&  apt-get update \
&&  apt-get install -y bash curl ca-certificates containerd make \
&&  mkdir -p /etc/containerd /opt/cni/bin /etc/cni/net.d \
&&  curl -L https://github.com/containernetworking/plugins/releases/download/v1.8.0/cni-plugins-linux-${TARGETARCH}-v1.8.0.tgz | tar -xz -C /opt/cni/bin \
&&  curl -L https://go.dev/dl/go1.24.3.linux-${TARGETARCH}.tar.gz | tar -xz -C /usr/local \
&&  make \
&&  curl -L https://github.com/containerd/nerdctl/releases/download/v2.2.0/nerdctl-2.2.0-linux-${TARGETARCH}.tar.gz  | tar -xz -C /usr/local/bin


FROM alpine:3.22.2
RUN apk add --no-cache iptables
COPY  --from=build /blipblop/src/bin/ /usr/local/bin/
COPY  --from=build /opt/cni /opt/cni
COPY  --from=build /etc/cni /etc/cni
