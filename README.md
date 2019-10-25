# Cloud Run API kubectl compatibility fix

**This is an experimental repository and is not meant to be used.**

This client-side tool patches the currently incomplete parts of the Cloud
Run (fully hosted) API (that is `{region}-run.googleapis.com`) by running a
proxy server locally.


This proxy will works by:

- Kubernetes discovery API requests (/api, /apis etc) are proxied to a
  Cloud Run on GKE cluster (hardcoded address) that offers them publicly.

- other requests about managing these resources are forwarded to
  us-central1-run.googleapis.com.

To run this proxy, simply compile and execute the Go program and copy the
CA certificate into the kubeconfig file. Running the program starts a local
kube-apiserver proxy at :6443 on HTTPS using a self-signed certificate (which
you need to trust in your kubeconfig).

Example kubeconfig file to point kubectl to this local proxy server:

```
apiVersion: v1
kind: Config
current-context: gcp_cloudrun_us-central1
clusters:
-  name: gcp_cloudrun_us-central1
   cluster:
    certificate-authority-data: "" # PASTE FROM THE COMMAND OUTPUT
    server: https://localhost:6443
contexts:
- name: gcp_cloudrun_us-central1
  context:
    cluster: gcp_cloudrun_us-central1
    namespace: ahmetb-samples-playground
    user: gcloud-user
users:
- name: gcloud-user
  user:
    auth-provider:
      name: gcp
      config:
        cmd-args: config config-helper --format=json
        cmd-path: gcloud
        expiry-key: '{.credential.token_expiry}'
        token-key: '{.credential.access_token}'
```

---

This is not an official Google project and is provided for demonstration
purposes. See [LICENSE](./LICENSE) for licensing information.
