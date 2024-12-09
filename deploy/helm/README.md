# Deploying Yukari on Kubernetes with Helm

If you don't have Helm installed already, install it from your favorite package manager:

```text
brew install helm
```

Add the Yukari helm repo:

```text
helm repo add yukari oci://ghcr.io/tigrisdata-community/helm/yukari
```

Create a Tigris bucket and [follow the Kubernetes quickstart directions](https://www.tigrisdata.com/docs/quickstarts/kubernetes/). Make sure you name the secret `yukari-tigris-creds`.

Then create a `values.yaml` file:

```yaml
ingress:
  className: "nginx" # or your cluster's ingress class name
  dnsName: "your.yukari.hostname.tld" # Change this to a domain name you control
  tls:
    secretName: your-yukari-hostname-tld-tls # Make this match your dnsName
  annotations:
    # If you use cert-manager with Let's Encrypt with a ClusterIssuer named letsencrypt-prod
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
```

Then install Yukari:

```text
helm install yukari yukari/yukari --namespace default -f values.yaml
```

And then use it with [Ollama](https://ollama.com). When you want to pull a model named `llama3.1`, instead of this command:

```text
ollama pull llama3.1
```

Use this command:

```text
ollama pull your.yukari.hostname.tld/library/llama3.1
```

You will also need to specify `your.yukari.hostname.tld/library/llama3.1` whenever you use the model via the API.
