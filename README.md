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
- [Deploy Prometheus](#deploy-prometheus)
- [Clean Up](#clean-up)


## Demo Background

_In this demo, we are going to deploy a go application (Fake Blog Server) that emits metrics to a Prometheus server. We will then use Grafana to visualize the metrics. We will incorportate Prometheus's remoteWrite functionality to demonstrate how metrics can be federated across environments. The first section `Instrumenting in Go 101` can be skipped if you just want to get to the interactive demo._

## Instrumenting in Go 101

This demo is mostly about Kubernetes, but we need to know how to instrument our application in Go to emit metrics. This part will be quick, but if you are interested in learning more about instrumenting your application in Go, check out the [`main.go`](main.go) where I have instrumented custom metrics for:
- `httpDuration`
- `responseStatus`
- `totalRequests`.

Prometheus has a lot of resources on how to instrument your application. Here is a link to get you started:
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
curl localhost:2112/metrics
```

Alright, now lets get to the fun stuff!! 

## Spin up a Kubernetes Cluster

Lets spin up a Kubernetes cluster using Kind. Kind is a tool for running local Kubernetes clusters using Docker container "nodes". Kind was chosen because it is easy to spin up and tear down a cluster. Kind is not meant for production use, but it is great for demos and testing.

In this case, we are going to map the host port 3333 to the container port 31388, which will be the `NodePort` where the application is exposed. This will allow the application's frontend to access the backend from our local machine.

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

Lets check to see if the application is running curling the health endpoint (The demo service is deployed as a `NodePort` service, so we can access it from our local machine, we have mapped 31388 which is the container port to 3333 which is the host port, which means we can curl it from our local machine):


```bash
curl localhost:3333/healthz | jq 
```

output
```json
{
  "alive": true
}
```


Open up the app in the browser by going to [`localhost:3333`](http://localhost:3333) and you should see the following:
## Clean Up