# Victoria Metrics

A simple task that runs Victoria Metrics time series database

```yaml
version: task/v1
meta:
  name: victoria-metrics
config:
  image: docker.io/victoriametrics/victoria-metrics:latest
  portMappings:
  - hostPort: 8428
    name: http
    targetPort: 8428
```
