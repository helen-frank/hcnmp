/*
Copyright helen-frank

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proxy

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/apis/core"

	"github.com/helen-frank/hcnmp/pkg/apis/cluster"
	"github.com/helen-frank/hcnmp/pkg/utils"
	"github.com/helen-frank/hcnmp/pkg/zone/clientset"
)

var (
	codeClusterClient sync.Map
	idClusterClient   sync.Map
	clusterWatch      watch.Interface
	retrySync         = make(chan struct{}, 1)
)

func InitProxy(clusterInfo, namespace, localClusterInfos string, kubeclient clientset.Interface) (err error) {
	watchcm(clusterInfo, namespace, localClusterInfos, kubeclient)

	if clusterWatch, err = kubeclient.CoreV1().ConfigMaps(namespace).Watch(context.Background(), metav1.SingleObject(metav1.ObjectMeta{
		Name:      clusterInfo,
		Namespace: namespace,
	})); err != nil {
		return err
	}

	go watchConfig(clusterInfo, namespace, localClusterInfos, kubeclient)

	return
}

func watchcm(clusterInfo, namespace, localClusterInfos string, kubeclient clientset.Interface) {
	// Rebuild until successful
	utilruntime.Must(wait.PollUntilContextCancel(context.Background(), time.Second, true, func(_ context.Context) (done bool, err error) {
		if _, err := kubeclient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), clusterInfo, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				if _, err = utils.CreateConfigMapsFromLocal(localClusterInfos, namespace, clusterInfo, kubeclient); err != nil {
					klog.Error(err)
					return false, err
				}
			} else {
				klog.Error(err)
				return false, err
			}
		}
		return true, nil
	}))
}

func watchConfig(clusterInfo, namespace, localClusterInfos string, kubeclient clientset.Interface) {
	retryNew := make(chan struct{}, 1)
	for {
		select {
		case event, ok := <-clusterWatch.ResultChan():
			if !ok {
				if err := newClustersAndWatch(clusterInfo, namespace, localClusterInfos, kubeclient); err != nil {
					klog.Error(err)
					retryNew <- struct{}{}
					return
				}
			} else {
				switch event.Type {
				case watch.Deleted, watch.Error:
					watchcm(clusterInfo, namespace, localClusterInfos, kubeclient)
					retryNew <- struct{}{}

				default:
					cm, err := utils.RuntimeToConfigMap(event.Object)
					if err != nil {
						klog.Error(err)
						retryNew <- struct{}{}
						return
					}

					if err := syncCodeClusterClient(clusterInfo, namespace, cm, kubeclient); err != nil {
						klog.Error(err)
						retrySync <- struct{}{}
						return
					}
				}

			}
		case <-retryNew:
			if err := newClustersAndWatch(clusterInfo, namespace, localClusterInfos, kubeclient); err != nil {
				klog.Error(err)
				retryNew <- struct{}{}
				return
			}
		case <-retrySync:
			if err := syncCodeClusterClient(clusterInfo, namespace, nil, kubeclient); err != nil {
				klog.Error(err)
				retrySync <- struct{}{}
				return
			}
		}
	}
}

func newClustersAndWatch(clusterInfo, namespace, localClusterInfos string, kubeclient clientset.Interface) (err error) {
	if _, err := kubeclient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), clusterInfo, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			if _, err = utils.CreateConfigMapsFromLocal(localClusterInfos, namespace, clusterInfo, kubeclient); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	clusterWatch, err = kubeclient.CoreV1().ConfigMaps(namespace).Watch(context.TODO(), metav1.SingleObject(metav1.ObjectMeta{
		Name:      clusterInfo,
		Namespace: namespace,
	}))

	return err
}

func syncCodeClusterClient(clusterInfo, namespace string, cm *corev1.ConfigMap, kubeclient clientset.Interface) (err error) {
	mu := sync.Mutex{}
	mu.Lock()
	defer mu.Unlock()

	if cm == nil {
		cm, err = kubeclient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), clusterInfo, metav1.GetOptions{})
		if err != nil {
			return err
		}
	}

	idtmp := make(map[string]*clientset.Clientset, len(cm.BinaryData))
	codetmp := make(map[string]*clientset.Clientset, len(cm.BinaryData))
	codes := make([]string, 0, len(cm.BinaryData))
	for code, data := range cm.BinaryData {
		client, err := newClient(data)
		if err != nil {
			return err
		}

		ns, err := client.CoreV1().Namespaces().Get(context.TODO(), core.NamespaceSystem, metav1.GetOptions{})
		if err != nil {
			return err
		}

		idtmp[string(ns.GetUID())] = client
		codetmp[code] = client
		codes = append(codes, code)
	}

	codeClusterClient.Range(func(key, _ any) bool {
		codeClusterClient.Delete(key)
		return true
	})

	idClusterClient.Range(func(key, _ any) bool {
		idClusterClient.Delete(key)
		return true
	})

	for k, v := range codetmp {
		codeClusterClient.Store(k, v)
	}

	for k, v := range idtmp {
		idClusterClient.Store(k, v)
	}
	klog.Infof("cluster %v proxy successfull", codes)
	return nil
}

func newClient(data []byte) (*clientset.Clientset, error) {
	clusterInfo := &cluster.ClusterInfo{}
	if err := utils.Std2Jsoniter.Unmarshal(data, clusterInfo); err != nil {
		return nil, err
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(clusterInfo.Kubeconfig)
	if err != nil {
		return nil, err
	}

	// set rateLimiter 1000
	restConfig.RateLimiter = flowcontrol.NewTokenBucketRateLimiter(1000, 1000)
	return clientset.NewForConfig(restConfig)
}

func GetClusterPorxyClientFromCode(code string) (*clientset.Clientset, error) {
	client, ok := codeClusterClient.Load(code)
	if !ok {
		return nil, fmt.Errorf("cluster %v Not Found", code)
	}
	return client.(*clientset.Clientset), nil
}

func GetClusterPorxyClientFromID(id string) (*clientset.Clientset, error) {
	client, ok := idClusterClient.Load(id)
	if !ok {
		return nil, fmt.Errorf("cluster %v Not Found", id)
	}
	return client.(*clientset.Clientset), nil
}
