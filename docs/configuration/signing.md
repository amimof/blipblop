[🏠 Home](/docs/README.md)

# Managing signing keys

voiyd-server uses ECDSA (P-256 / ES256) keys for signing and verifying JWT tokens. Currently, their primary purpose is to back the lease service with a signing and validation mechanism. If neither `--jwt-signing-key` nor `--jwt-verification-key` is provided, then voiyd-server will automatically generate an ECDSA private/public key pair at startup. Note that any signed JWT tokens signed will be instantly invalid if the server restarts. If you want persistent keys then you need to generate these and provide them to `voiyd-server` on the command line.

## Generating ECDSA keys with OpenSSL (ES256)

1. Generate private key used for signing

   ```bash
   openssl ecparam -name prime256v1 -genkey -noout -out ec256-private.pem
   ```

  This create a PEM-encoded private key file

1. Generate the corresponding public key used for verification

   ```bash
   openssl ec -in ec256-private.pem -pubout -out ec256-public.pem
   ```

   This create a PEM-encoded public key file. The public key can be used to verify any tokens signed by the private key for validity.

## Configuring voiyd-server with custom keys

To have voiyd-server use custom signing and verification keys, simply provide `--jwt-signing-key` and `--jwt-verification-key` when starting the server. For example:

```bash
voiyd-server \
  --jwt-signing-key=ec245-private.pem \
  --jwt-verification-key=ec245-public.pem
```
