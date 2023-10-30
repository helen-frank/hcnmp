# hcnmp (Helen Cloud Native Management Platform)

## 1. 简述
一个简单、高效、可靠的多Kubernetes集群云原生管理平台demo项目

## 2. 快速开始
1. kind creates a cluster
```shell
kind create cluster --name=multi-node
```

2. 获取kubeconfig并粘贴到 `.config/kube.config`
```shell
kubectl config view --raw
```

3. 运行

> 本地运行
```shell
go run ./main.go --kubeconfig .config/kube.config
```

> 集群部署
```shell
kubectl apply -f sample/hcnmp.yaml
```

4. 测试接口

导入[api文档](hcnmp.openapi.json)到apifox, 配置apifox访问接口

## 3. hcnmp 优秀的机制
### 多集群client缓存机制
使用POST /apis/cluster/v1/code/{clusterCode} 为hcnmp增加集群后, hcnmp内部会把集群数据写向一个configmap, 然后各个hcnmp通过 watch 这个configmap监听到了变动事件, 然后从事件里拿到这个configmap, 读取configmap的数据生成client放入自身的sync.Map里, 在业务接口需要操作集群时, 从sync.Map里获取对应的集群client

该机制利用configmap的list/watch机制, 可在多个hcnmp副本间实现集群数据一致性
