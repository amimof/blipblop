# TLS Certificates for voiyd

This directory contains example certificate configuration and helper commands for running voiyd components (server and nodes) over TLS.

> The files in this directory are intended primarily for **local development and testing**

<!--toc:start-->
- [TLS Certificates for voiyd](#tls-certificates-for-voiyd)
  - [Server Certificates](#server-certificates)
    - [Generate CA Certificates](#generate-ca-certificates)
    - [Generate a Server Certificate](#generate-a-server-certificate)
  - [Configure voiyd to Use TLS](#configure-voiyd-to-use-tls)
    - [Configure voiyd-server](#configure-voiyd-server)
    - [Configure voiyd-node](#configure-voiyd-node)
    - [Configure voiydctl](#configure-voiydctl)
<!--toc:end-->

## Server Certificates

> Keep private keys (`ca.key`, `server.key`) secure and limit access to them.
>
### Generate CA Certificates

```bash
# Generate a 2048-bit private key for the CA
openssl genrsa -out ca.key 2048

# Generate a self-signed CA certificate
openssl req -x509 -new -days 365 -sha256 -nodes \
  -key ca.key \
  -out ca.crt \
  -config ca.conf
```

This will create `ca.key` and `ca.crt` in the current directory.

### Generate a Server Certificate

```bash
# Generate a 2048-bit server private key
openssl genrsa -out server.key 2048

# Generate a certificate signing request (CSR) using server.conf
openssl req -new -sha256 \
  -key server.key \
  -out server.csr \
  -config server.conf

# Sign the CSR with the local CA to produce server.crt
openssl x509 -req -sha256 -days 365 \
  -in server.csr \
  -CA ca.crt \
  -CAkey ca.key \
  -CAcreateserial \
  -out server.crt \
  -extfile server.conf \
  -extensions v3_req
```

This will create `server.key`, `server.csr`, `server.crt`, and (if not present) `ca.srl`.

## Configure voiyd to Use TLS

Configuration method may varry depending on how installed voiyd. Generally you want to run it with systemd. If you used the automated installer then systemd is used by default. To edit the systemd unit files after installation you may run `systemctl edit voiyd-server/voiyd-node`

### Configure voiyd-server

Configure the following flags on the `voiyd-server` executable.

| Flag | Value |
|--------|-------------|
| `--tls-key` | `server.key` |
| `--tls-certificate` | `server.crt` |
| `--tls-ca` | `ca.crt` |

### Configure voiyd-node

Configure the following flags on `voiyd-node`

| Flag | Value |
|--------|-------------|
| `--tls-key` | `server.key` |
| `--tls-certificate` | `server.crt` |
| `--tls-ca` | `ca.crt` |

### Configure voiydctl

If you haven't initialized a voiydctl config yet:

```bash
voiydctl config create-server dev-cluster --address localhost:5743 --tls --ca ca.crt
```

If a context with your server already exist then open `~/.voiyd/voiydctl.yaml` and replace the CA block.
