# blipblop
Distributed `containerd` workloads

## System Requirements

Server
* Go

Node
* Containerd >= 1.4.13
* Iptables

## Try it out
```
go get -u github.com/amimof/blipblop/cmd/blipblop
blipblop --tls-key certs/server-key.pem --tls-certificate certs/server.pem
```