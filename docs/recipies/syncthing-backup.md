# Syncthing

I use this configuration to make backups in my homelab. I setup two syncthing instances and bind them to nodes using a `nodeSelector`. This example uses local disk but you can also create a `Volume` and use that instead.

```yaml
---
version: task/v1
meta:
  name: syncthing-remote
config:
  capabilities: {}
  nodeSelector:
    role: syncthing-remote
  envvars:
    - name: PUID
      value: "1000"
    - name: PGID
      value: "1000"
  image: docker.io/syncthing/syncthing:latest
  mounts:
    - destination: /var/syncthing
      name: data
      # volume: syncthing-data
      source: /var/lib/syncthing
  portMappings:
    - hostPort: 8384
      name: http
      targetPort: 8384
    - hostPort: 22000
      name: tcp
      targetPort: 22000
---
version: task/v1
meta:
  name: syncthing-local
config:
  capabilities: {}
  nodeSelector:
    role: syncthing-local
  envvars:
    - name: PUID
      value: "0"
    - name: PGID
      value: "0"
  image: docker.io/syncthing/syncthing:latest
  mounts:
    - destination: /var/syncthing
      name: syncthing-data
      source: /var/lib/syncthing
    - destination: /var/syncthing/victoria-metrics-data
      name: victoria-metrics-data
      source: /var/lib/containers/storage/volumes
  portMappings:
    - hostPort: 8384
      name: 8384:8384
      targetPort: 8384
    - hostPort: 22000
      name: tcp
      targetPort: 22000

```
