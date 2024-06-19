# blipblop

Distributed `containerd` workloads

## System Requirements

- Linux
- Containerd >= 1.4.13
- Iptables
- [CNI plugins](https://github.com/containernetworking/plugins)

## Try it out

Run a server & node instance on localhost

```shell
make run
```

List nodes in the cluster

```json
curl http://localhost:8443/api/v1/nodes
{
  "node": {
    "name": "devnode",
    "labels": {},
    "created": "2023-05-14T09:08:08.074716028Z",
    "updated": "2023-05-14T09:08:08.074716294Z",
    "revision": "1",
    "status": {
      "ips": [
        "127.0.0.1/8",
        "::1/128",
        "192.168.13.19/24",
        "fe80::526b:8dff:feee:b3b5/64",
        "10.69.0.1/16",
        "fe80::c87:deff:fec5:777b/64"
      ],
      "hostname": "amir-lab",
      "arch": "amd64",
      "os": "linux",
      "ready": true
    }
  }
}
```

## Development Environment

To start developing you need to install following on a `Linux` host.

- [Go](https://go.dev/doc/install) > 1.19
- [protoc](https://grpc.io/docs/protoc-installation/) v3
- [protoc-gen-go](https://grpc.io/docs/languages/go/quickstart/) >= @v1.28

  ```shell
  go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
  ```

- [protoc-gen-go-grpc](https://grpc.io/docs/languages/go/quickstart/) >= v1.2

  ```shell
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2
  ```

- [protoc-gen-grpc-gateway](https://github.com/grpc-ecosystem/grpc-gateway) >= v2.19.1

  ```shell
  go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.19.1
  ```

- [protoc-gen-openapiv2](https://github.com/grpc-ecosystem/grpc-gateway) >= v2.19.1

  ```shell
  go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@v2.19.1
  ```

- [buf](https://buf.build/docs/installation) => v1.30.0

  ```
  go install github.com/bufbuild/buf/cmd/buf@v1.30.0
  ```

Update your `PATH` so that the `protoc` compiler can find the plugins

```shell
export PATH="$PATH:$(go env GOPATH)/bin"
```
