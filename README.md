# hcnmp (Helen Cloud Native Management Platform)
[简体中文](./docs/READMD_cn.md)

## 1. Brief description
A simple, efficient, and reliable cloud-native management platform demo project for multiple Kubernetes clusters

## 2. Quick start
1. kind creates a cluster
```shell
kind create cluster --name=multi-node
```

2. Get the kubeconfig and paste it into `.config/kube.config`.

```shell
kubectl config view --raw
```

3. Run

> Run locally
```shell
go run ./main.go --kubeconfig .config/kube.config
```

> Cluster deployment
```shell
kubectl apply -f sample/hcnmp.yaml
```

4. Testing the interface

Import [api documentation](./docs/hcnmp.openapi.json) to apifox, configure apifox access interface

## 3. hcnmp Excellent mechanism
### Multi-cluster client caching mechanism
After using POST /apis/cluster/v1/code/{clusterCode} to add a cluster for hcnmp, hcnmp will write the cluster data to a configmap internally, and then each hcnmp listens to the change event by watching this configmap, and then gets the configmap from the event. configmap, read the configmap data to generate clients into its own sync.Map, when the business interface needs to operate the cluster, get the corresponding cluster clients from sync.

This mechanism utilizes the list/watch mechanism of configmap to achieve cluster data consistency among multiple hcnmp replicas.
