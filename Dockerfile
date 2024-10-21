FROM ubuntu:24.10

COPY . /blipblop/src/
WORKDIR /blipblop/src
ENV PATH=$PATH:/usr/local/go/bin

RUN apt-get update \
&&  apt-get install -y bash curl ca-certificates containerd build-essential \
&&  mkdir -p /etc/containerd /opt/cni/bin \
&&  curl -L https://github.com/containernetworking/plugins/releases/download/v1.6.0/cni-plugins-linux-amd64-v1.6.0.tgz | tar -xz -C /opt/cni/bin \
&&  curl -L https://go.dev/dl/go1.23.2.linux-amd64.tar.gz | tar -xz -C /usr/local \
&&  make \
&&  mv bin/* /usr/local/bin/ 

CMD ["/blipblop/src/entrypoint.sh"]

EXPOSE 2375
