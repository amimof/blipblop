# TLS related stuff for testing purpose only

## CA certificates

```
openssl genrsa -out ca.key 2048
openssl req -x509 -new -days 365 -sha256 -nodes -key ca.key -out ca.crt -config ca.conf
```

## Server certificates

```
openssl genrsa -out server.key 2048
openssl req -new -sha256 \
    -key server.key \
    -out server.csr \
    -config server.conf
openssl x509 -req -sha256 -days 365 -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt -extfile server.conf -extensions v3_req
```
