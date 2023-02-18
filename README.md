# Kubernetes Metrics Workshop


**Prereqs**
- [Kind](https://kind.sigs.k8s.io/) installed on your machine
- [Docker](https://docker.com) or tool to build container images
- [Dockerhub](https://hub.docker.com/) or other image registry account
- [Go](https://go.dev/) version 1.19 or higher (Not pertinent to workshop, but needed for the demo)

**TOC**
- [Demo Background](#demo-background)
- [Instrumenting in Go 101](#instrumenting-in-go-101)
- [Spin up a Kubernetes Cluster](#spin-up-a-kubernetes-cluster)
- [Deploy the Demo App](#deploy-the-demo-app)
- [Deploy Prometheus Operator](#deploy-prometheus-operator)
- [Configure Prometheus](#configure-prometheus)
- [Clean Up](#clean-up)


## Demo Background

_In this demo, we are going to deploy a go application (Fake Blog Server) that emits metrics to a Prometheus server. We are going to look at several key features of Prometheus, including `externalLabels` to identify metrics in a federated environment, `remoteWrite` to demonstrate how metrics can be federated across environments, `prometheusRules` to define and trigger alert, and `serviceMonitors` to tell Prometheus what to scrape. The first section is `Instrumenting in Go 101`, and can be skipped if you just want to skip to the interactive demo._

## Instrumenting in Go 101

This demo is mostly about Kubernetes, but we need to know how to instrument our application in Go to emit metrics. This part will be quick, but if you are interested in learning more about instrumenting your application in Go, check out the [`main.go`](main.go) where I have instrumented custom metrics for:
- `httpDuration`
- `responseStatus`
- `totalRequests`.

I should also note that I have created an endpoint in the app that can receive metrics from Prometheus's remoteWrite endpoint, so our application can receive metrics, and it also emits metrics. This is useful for testing purposes, but in reality, you would only want to emit metrics from your application.


```go
func handleMetrics(w http.ResponseWriter, r *http.Request) {
	req, err := remote.DecodeWriteRequest(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, ts := range req.Timeseries {
		m := make(model.Metric, len(ts.Labels))
		for _, l := range ts.Labels {
			m[model.LabelName(l.Name)] = model.LabelValue(l.Value)
		}
		fmt.Println(m)

		for _, s := range ts.Samples {
			fmt.Printf("\tSample:  %f %d\n", s.Value, s.Timestamp)
		}

		for _, e := range ts.Exemplars {
			m := make(model.Metric, len(e.Labels))
			for _, l := range e.Labels {
				m[model.LabelName(l.Name)] = model.LabelValue(l.Value)
			}
			fmt.Printf("\tExemplar:  %+v %f %d\n", m, e.Value, e.Timestamp)
		}
	}
}
```

Prometheus has resources on how to instrument your application. Here is a link to get you started:
- [Prometheus Website](https://prometheus.io/docs/guides/go-application/)


Essentially, instrumenting an application in Go is as simple as importing the prometheus library and adding a few lines of code. The below example shows how to expose the metrics endpoint with default metrics, which are honestly enough in some cases because this allows you to use the `up` metric which tells you if your application is up or down.

```go
package main

import (
        "net/http"

        "github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
        http.Handle("/metrics", promhttp.Handler())
        http.ListenAndServe(":2112", nil)
}
```

This is a great start, but in reality, we need custom metrics. The below example shows how to add a custom metric `myapp_processed_ops_total` to the metrics endpoint which will increase by 1 every 2 seconds.

```go
package main

import (
        "net/http"
        "time"

        "github.com/prometheus/client_golang/prometheus"
        "github.com/prometheus/client_golang/prometheus/promauto"
        "github.com/prometheus/client_golang/prometheus/promhttp"
)

func recordMetrics() {
        go func() {
                for {
                        opsProcessed.Inc()
                        time.Sleep(2 * time.Second)
                }
        }()
}

var (
        opsProcessed = promauto.NewCounter(prometheus.CounterOpts{
                Name: "myapp_processed_ops_total",
                Help: "The total number of processed events",
        })
)

func main() {
        recordMetrics()

        http.Handle("/metrics", promhttp.Handler())
        http.ListenAndServe(":2112", nil)
}
```

to run the application, run the following command:

```bash
go run main.go
```

and to access the metrics endpoint, run the following command:

```bash
curl localhost:2112/api/metrics
```

Alright, now lets get to the fun stuff!! 

## Spin up a Kubernetes Cluster

Lets spin up a Kubernetes cluster using Kind. Kind is a tool for running local Kubernetes clusters using Docker container "nodes". Kind was chosen because it is easy to spin up and tear down a cluster. Kind is not meant for production use, but it is great for demos and testing.

In this case, we are going to map the host port 8080 to the container port 31388, which will be the `NodePort` where the application is exposed. This will allow the application's frontend to access the backend from our local machine.

To spin up a cluster, run the following command:

```bash
cat <<EOF | kind create cluster --name=prom-demo --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 31388
    hostPort: 8080
    protocol: TCP
EOF
```

## Deploy the Demo App


Now that we have a cluster, lets deploy the demo app. The demo app is a simple demo blog.

```bash
kubectl apply -f k8s/
kubectl config set-context $(kubectl config current-context) --namespace=demo
```

expected output
```bash
namespace/demo created
deployment.apps/blog created
service/blog created
Context "kind-prom-demo" modified.
```

Wait for the pod to become ready   
  
```bash
kubectl wait --for=condition=Ready pod -l app=blog --timeout=4m 
```

Lets check to see if the application is running curling the health endpoint (The demo service is deployed as a `NodePort` service, so we can access it from our local machine, we have mapped 31388 which is the container port to 8080 which is the host port, which means we can curl it from our local machine):


```bash
curl localhost:8080/api/healthz | jq 
```

output
```json
{
  "alive": true
}
```


Open up the app in the browser by going to [`localhost:8080`](http://localhost:8080) and you should see the following:

![img](frontend.png)

## Deploy Prometheus Operator

Now that we have a cluster and a demo app, let's deploy Prometheus Operator. Prometheus Operator is a Kubernetes Operator that creates, configures, and manages Prometheus instances in Kubernetes. It is a great tool for deploying Prometheus in Kubernetes. 

```bash
kubectl create -f https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/main/bundle.yaml 
```

Wait for the operator to be ready

```bash
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/name=prometheus-operator --timeout=4m -n default
```

## Configure Prometheus

Now that we have the operator installed, we are going to deploy a `Prometheus` instance. The `Prometheus` instance will be configured to remoteWrite metrics to the demo app. 

```yaml
kubectl create -f -<<EOF
kind: Prometheus
apiVersion: monitoring.coreos.com/v1
metadata:
  name: k8s
  namespace: default
spec:
  # all prometheusRules
  ruleSelector: {}
  # all serviceMontiors in the namespace
  serviceMonitorSelector: {}
  # all namespaces
  serviceMonitorNamespaceSelector: {}
  logLevel: debug
  logFormat: json
  replicas: 1
  image: quay.io/prometheus/prometheus:v2.35.0
  serviceAccountName: prometheus-operator 
  remoteWrite:
    - url: http://$(kubectl get no prom-demo-control-plane -ojsonpath='{.status.addresses[0].address}'):31388/api/remote # Demo app to remoteRead endpoint through nodePort
  resources:
    requests:
      memory: 400Mi
EOF
```

Wait for the operator to spin up the Prometheus instance

```bash
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/instance=k8s --timeout=4m -n default
```

Next, we need to give the `serviceAccount` `prometheus-operator` permission to scrape `Endpoints`, `Services`, and `Pods`. 

First, create the role

```yaml
kubectl apply -f -<<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: prom-scrape
rules:
- apiGroups:
  - ""
  resources:
  - pods
  - endpoints
  - services
  verbs:
  - get
  - watch
  - list
EOF
```

Now, bind it to the service account `prometheus-operator` in the default namespace

```bash
kubectl apply -f -<<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  creationTimestamp: null
  name: prom-scrape-binding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: prom-scrape
subjects:
- kind: ServiceAccount
  name: prometheus-operator
  namespace: default
EOF
```

Make sure that the `ServiceAccount` `prometheus-operator` has the correct permissions

```bash
kubectl auth can-i get pods --as=system:serviceaccount:default:prometheus-operator -n demo
kubectl auth can-i get services --as=system:serviceaccount:default:prometheus-operator -n demo
kubectl auth can-i get endpoints --as=system:serviceaccount:default:prometheus-operator -n demo
```

expected output

```bash
yes
yes
yes
```

Now it is time to tell Prometheus what to scrape, this is where the `ServiceMonitor` come in. We tell prometheus to scrape at `/api/metrics` on the `http` port of the service with label `app: blog`. To be 100% clear, this is the service you are scraping...

```bash
k get svc -n demo -l app=blog
```

output
```
NAME   TYPE       CLUSTER-IP    EXTERNAL-IP   PORT(S)          AGE
blog   NodePort   10.96.73.85   <none>        8080:31388/TCP   179m
```

Now go ahead and create the service monitor

```yaml
kubectl apply -f -<<EOF
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  labels:
    app: blog
  name: blog
  namespace: default
spec:
  endpoints:
  - port: http
    path: /api/metrics
  namespaceSelector:
    matchNames:
    - demo
  selector:
    matchLabels:
      app: blog
EOF
```


Now, lets verify that Prometheus is collecting metrics from our demo blog app

```bash
kubectl port-forward svc/prometheus-operated 9090 -n default
```

Open [localhost:9090](http://localhost:9090) in the browser.

Next Click `Status` -> `Targets` and you should see the following:

![targets](targets.png)


Okay, now we know that  our app is being scraped.

Next lets look at the metrics that our blog is collecting. Click `Graph` and type in `{container="blog"}` and you should see the following:

![metrics](metrics.png)

Next, lets look at the total number of requests to the `/` path, which is the frontend.

In the input, add `http_requests_total{container="blog", path="/"}`

```bash
http_requests_total{container="blog", endpoint="http", instance="10.244.0.11:8080", job="blog", metrics="custom", namespace="demo", path="/", pod="blog-f46cc88fb-smwp5", service="blog"}
```

the last metric I want to checkout is `up`, which is a metric that is collected by Prometheus by default. This metric is a great way to check if the service is up or down. 

In the input, add `up{container="blog"}`

```bash
up{container="blog", endpoint="http", instance="10.244.0.11:8080", job="blog", namespace="demo", pod="blog-f46cc88fb-smwp5", service="blog"}
1
```


Next, we are going to deploy a `PrometheusRule`, define some custom rules and alerts.l alert us if the `up` metric is `0` for 5 minutes. 

```yaml
kubectl apply -f -<<EOF
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule 
metadata:
  labels:
    app: blog
  name: blog
  namespace: default
spec:
  groups:
  - name: blog_custom_rules
    # In this case, we need to trigger an alert as soon as an instances goes down for demo, 15s too long
    interval: 1s # Configurable like doc says 
    rules: 

    # The average memory used by all queries over a given time period
    - record: blog_up
      expr: up{container="blog"} == 1

    # The max memory available to queries cluster wide
    - record: twentieth_visitor
      expr: http_requests_total{container="blog", path="/"} >= 20


  - name: starburst_alert_rules
    rules: 
 
    # Instance down for 1 minute
    - alert: blog_down
      expr: absent(blog_up)
      for: 1m
      labels:
        severity: page
      annotations:
        summary: "Blog instance down for 1 min"
        description: "Instance of the blog is down" 

    # 20th Visitor
    - alert: twentieth_visitor
      expr: twentieth_visitor
      labels:
        severity: page
      annotations:
        summary: "20th Visitor Alert"
        description: "Since deployed, the 20th visitor has hit the blog" 
EOF
```


Lets talk about external Labels. External labels are labels that are added to every metric that is collected by Prometheus. This is a great way to add context to your metrics. When you are federating metrics from one cluster to another, and you have many metrics with the same name, you can use external labels to differentiate between them, for instnace, you can add an external labek `cluster: dev` to all metrics collected from the dev cluster. 


What about debugging? Well, you can use the []`promtool`](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/#configuring-rules) to check if your rules are valid. Another way, is to check the logs of the prometheus operator. 

```bash
k logs -f -l app.kubernetes.io/name=prometheus-operator -n default
```

If you had misconfigured a rule or serviceMonitor, you would get an error in the logs

```bash
k logs -f -l app.kubernetes.io/name=prometheus-operator -n default | grep l
evel=error
```

output
```bash
level=error ts=2023-02-18T14:49:35.199449956Z caller=klog.go:116 component=k8s_client_runtime func=ErrorDepth msg="sync \"default/k8s\" failed: Invalid rule"
level=error ts=2023-02-18T14:50:42.19384299Z caller=klog.go:116 component=k8s_client_runtime func=ErrorDepth msg="sync \"default/k8s\" failed: Invalid rule"
```

## Clean Up