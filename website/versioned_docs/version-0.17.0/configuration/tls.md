---
title: TLS and certificates
---

## TLS configuration

By default the dashboard will listen on 0.0.0.0:9001 with TLS disabled and
without exposing any external connection.

Exposing services without TLS if not recommended. Without a certificate, a user
can't be sure they are using the right service, and the traffic will be easily
monitored, or even tampered with. All communication between the user and an endpoint
with TLS will be encrypted.

To expose an external connection, you must first configure TLS. TLS termination
can be provided via an ingress controller or directly by the dashboard. In
either case, the helm release must be updated. To have the dashboard itself
handle TLS, you must create a `tls` secret containing the cert and key:

```cli
kubectl create secret tls my-tls-secret \
  --cert=path/to/cert/file \
  --key=path/to/key/file
```

and reference it from the helm release:

```yaml
  values:
    serverTLS:
      enabled: true
      secretName: "my-tls-secret"
```

If you prefer to delegate TLS handling to the ingress controller instead, your
helm release should look like:

```yaml
  values:
    ingress:
      enabled: true
      ... other parameters specific to the ingress type ...
```

## cert-manager

Install [cert-manager](../guides/cert-manager.md) and request a `Certificate` in
the `flux-system` namespace. Provide the name of secret associated with the
certificate to the weave-gitops-enterprise HelmRelease as described above.
