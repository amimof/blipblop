# blipblop
Distributed `containerd` workloads

  ## System Requirements
  * Linux
  * Go > 1.19
  * Containerd >= 1.4.13
  * Iptables

  ## Try it out
  Run a server & node instance on localhost
  ```
make run
```

List nodes in the cluster
```
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