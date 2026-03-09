# Victoria Metrics

A simple task that runs Victoria Metrics time series database

```yaml
version: task/v1
meta:
  name: vm1
config:
  capabilities: {}
  envvars:
    - name: PUID
      value: "1000"
    - name: PGID
      value: "1000"
  image: docker.io/victoriametrics/victoria-metrics:latest
  mounts:
    - destination: /data
      name: data
      source: /tmp
  portMappings:
    - hostPort: 8384
      name: http
      targetPort: 8384
    - hostPort: 22000
      name: tcp
      targetPort: 22000
```
