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

package clusters

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	api "k8s.io/kubernetes/pkg/apis/core"

	"github.com/helen-frank/hcnmp/pkg/apis/cluster"
	"github.com/helen-frank/hcnmp/pkg/server/servererror"
	"github.com/helen-frank/hcnmp/pkg/utils"
	"github.com/helen-frank/hcnmp/pkg/zone/clientset"
	"github.com/helen-frank/hcnmp/pkg/zone/proxy"
)

func (h *handler) addCluster(c *gin.Context) {
	clusterCode := c.Param("clusterCode")
	if _, err := proxy.GetClusterPorxyClientFromCode(clusterCode); err == nil {
		servererror.HandleError(c, http.StatusConflict, fmt.Errorf("cluster %v existed", clusterCode))
		return
	}

	kubeconfig, err := io.ReadAll(c.Request.Body)
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	// code existed
	cm, err := h.client.CoreV1().ConfigMaps(h.namespace).Get(context.TODO(), h.clusterInfos, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if cm, err = utils.CreateConfigMapsFromLocal(h.localClusterInfos, h.namespace, h.clusterInfos, h.client); err != nil {
				servererror.HandleError(c, http.StatusInternalServerError, err)
				return
			}
		} else {
			servererror.HandleError(c, http.StatusInternalServerError, err)
			return
		}
	}
	if cm.BinaryData == nil {
		cm.BinaryData = make(map[string][]byte)
	}

	if _, ok := cm.BinaryData[clusterCode]; ok {
		servererror.HandleError(c, http.StatusConflict, fmt.Errorf("cluster %v existed", clusterCode))
		return
	}

	// uid existed
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}
	ns, err := client.CoreV1().Namespaces().Get(context.TODO(), api.NamespaceSystem, metav1.GetOptions{})
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	for code, data := range cm.BinaryData {
		clusterInfo := &cluster.ClusterInfo{}
		if err := utils.Std2Jsoniter.Unmarshal(data, clusterInfo); err != nil {
			servererror.HandleError(c, http.StatusInternalServerError, err)
			return
		}
		if clusterInfo.ID == string(ns.GetUID()) {
			servererror.HandleError(c, http.StatusConflict, fmt.Errorf("cluster %v existed", code))
			return
		}
	}

	clusterInfo := cluster.ClusterInfo{
		ID:         string(ns.UID),
		Code:       clusterCode,
		Kubeconfig: kubeconfig,
	}

	if data, err := utils.Std2Jsoniter.Marshal(clusterInfo); err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	} else {
		cm.BinaryData[clusterCode] = data
	}

	if _, err := h.client.CoreV1().ConfigMaps(h.namespace).Update(context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, nil)
}

func (h *handler) removeCluster(c *gin.Context) {
	clusterCode := c.Param("clusterCode")

	_, err := proxy.GetClusterPorxyClientFromCode(clusterCode)
	if err != nil {
		servererror.HandleError(c, http.StatusNotFound, err)
		return
	}

	// code existed
	cm, err := h.client.CoreV1().ConfigMaps(h.namespace).Get(context.TODO(), h.clusterInfos, metav1.GetOptions{})
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}
	if cm.BinaryData == nil {
		servererror.HandleError(c, http.StatusNotFound, fmt.Errorf("clusterinfo %v is empty", h.clusterInfos))
		return
	}

	if _, ok := cm.BinaryData[clusterCode]; !ok {
		servererror.HandleError(c, http.StatusNotFound, fmt.Errorf("cluster %v Not Found", clusterCode))
		return
	}

	delete(cm.BinaryData, clusterCode)

	if _, err := h.client.CoreV1().ConfigMaps(h.namespace).Update(context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, nil)
}

func (h *handler) getCluster(c *gin.Context) {
	clusterCode := c.Param("clusterCode")
	_, err := proxy.GetClusterPorxyClientFromCode(clusterCode)
	if err != nil {
		servererror.HandleError(c, http.StatusNotFound, err)
		return
	}

	cm, err := h.client.CoreV1().ConfigMaps(h.namespace).Get(context.TODO(), h.clusterInfos, metav1.GetOptions{})
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}
	if cm.BinaryData == nil {
		servererror.HandleError(c, http.StatusNotFound, fmt.Errorf("clusterinfo %v is empty", h.clusterInfos))
		return
	}

	data, ok := cm.BinaryData[clusterCode]
	if !ok {
		servererror.HandleError(c, http.StatusNotFound, fmt.Errorf("cluster %v Not Found", clusterCode))
		return
	}

	clusterInfo := &cluster.ClusterInfo{}
	if err := utils.Std2Jsoniter.Unmarshal(data, clusterInfo); err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, clusterInfo)
}

func (h *handler) getClusters(c *gin.Context) {
	cm, err := h.client.CoreV1().ConfigMaps(h.namespace).Get(context.TODO(), h.clusterInfos, metav1.GetOptions{})
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}
	if cm.BinaryData == nil {
		c.JSON(http.StatusOK, []*cluster.ClusterInfo{})
		return
	}

	clusterInfos := make([]*cluster.ClusterInfo, 0, len(cm.BinaryData))

	for _, v := range cm.BinaryData {
		clusterInfo := &cluster.ClusterInfo{}
		if err := utils.Std2Jsoniter.Unmarshal(v, clusterInfo); err != nil {
			servererror.HandleError(c, http.StatusInternalServerError, err)
			return
		}
		clusterInfos = append(clusterInfos, clusterInfo)
	}

	c.JSON(http.StatusOK, clusterInfos)
}

func (h *handler) updateCluster(c *gin.Context) {
	clusterCode := c.Param("clusterCode")
	if _, err := proxy.GetClusterPorxyClientFromCode(clusterCode); err != nil {
		servererror.HandleError(c, http.StatusNotFound, err)
		return
	}

	kubeconfig, err := io.ReadAll(c.Request.Body)
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	// code existed
	cm, err := h.client.CoreV1().ConfigMaps(h.namespace).Get(context.TODO(), h.clusterInfos, metav1.GetOptions{})
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}
	if cm.BinaryData == nil {
		servererror.HandleError(c, http.StatusNotFound, fmt.Errorf("clusterinfo %v is empty", h.clusterInfos))
		return
	}

	if _, ok := cm.BinaryData[clusterCode]; !ok {
		servererror.HandleError(c, http.StatusNotFound, fmt.Errorf("cluster %v Not Found", clusterCode))
		return
	}

	if cm.BinaryData != nil {
		// no change in preprocessed cluster information
		data := cm.BinaryData[clusterCode]
		clusterInfo := &cluster.ClusterInfo{}
		if err := utils.Std2Jsoniter.Unmarshal(data, clusterInfo); err != nil {
			servererror.HandleError(c, http.StatusInternalServerError, err)
			return
		}

		if bytes.Equal(clusterInfo.Kubeconfig, kubeconfig) {
			c.JSON(http.StatusOK, nil)
			return
		}

		delete(cm.BinaryData, clusterCode)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}
	ns, err := client.CoreV1().Namespaces().Get(context.TODO(), api.NamespaceSystem, metav1.GetOptions{})
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	for code, data := range cm.BinaryData {
		clusterInfo := &cluster.ClusterInfo{}
		if err := utils.Std2Jsoniter.Unmarshal(data, clusterInfo); err != nil {
			servererror.HandleError(c, http.StatusInternalServerError, err)
			return
		}
		if clusterInfo.ID == string(ns.GetUID()) {
			servererror.HandleError(c, http.StatusConflict, fmt.Errorf("cluster %v existed", code))
			return
		}
	}

	clusterInfo := cluster.ClusterInfo{
		ID:         string(ns.UID),
		Code:       clusterCode,
		Kubeconfig: kubeconfig,
	}

	if newData, err := utils.Std2Jsoniter.Marshal(clusterInfo); err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	} else {
		cm.BinaryData[clusterCode] = newData
	}

	if _, err := h.client.CoreV1().ConfigMaps(h.namespace).Update(context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, nil)
}

func (h *handler) applyCluster(c *gin.Context) {
	clusterCode := c.Param("clusterCode")
	if _, err := proxy.GetClusterPorxyClientFromCode(clusterCode); err != nil {
		klog.Warning(err)
	}

	kubeconfig, err := io.ReadAll(c.Request.Body)
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	// code existed
	cm, err := h.client.CoreV1().ConfigMaps(h.namespace).Get(context.TODO(), h.clusterInfos, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if cm, err = utils.CreateConfigMapsFromLocal(h.localClusterInfos, h.namespace, h.clusterInfos, h.client); err != nil {
				servererror.HandleError(c, http.StatusInternalServerError, err)
				return
			}
		} else {
			servererror.HandleError(c, http.StatusInternalServerError, err)
			return
		}
	}
	if cm.BinaryData == nil {
		cm.BinaryData = make(map[string][]byte)
	} else if data, ok := cm.BinaryData[clusterCode]; ok {
		// no change in preprocessed cluster information
		clusterInfo := &cluster.ClusterInfo{}
		if err := utils.Std2Jsoniter.Unmarshal(data, clusterInfo); err != nil {
			servererror.HandleError(c, http.StatusInternalServerError, err)
			return
		}

		if bytes.Equal(clusterInfo.Kubeconfig, kubeconfig) {
			c.JSON(http.StatusOK, nil)
			return
		}

		delete(cm.BinaryData, clusterCode)
	}

	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}
	client, err := clientset.NewForConfig(restConfig)
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}
	ns, err := client.CoreV1().Namespaces().Get(context.TODO(), api.NamespaceSystem, metav1.GetOptions{})
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	for code, data := range cm.BinaryData {
		clusterInfo := &cluster.ClusterInfo{}
		if err := utils.Std2Jsoniter.Unmarshal(data, clusterInfo); err != nil {
			servererror.HandleError(c, http.StatusInternalServerError, err)
			return
		}
		if clusterInfo.ID == string(ns.GetUID()) {
			servererror.HandleError(c, http.StatusConflict, fmt.Errorf("cluster %v existed", code))
			return
		}
	}

	clusterInfo := cluster.ClusterInfo{
		ID:         string(ns.UID),
		Code:       clusterCode,
		Kubeconfig: kubeconfig,
	}

	if newData, err := utils.Std2Jsoniter.Marshal(clusterInfo); err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	} else {
		cm.BinaryData[clusterCode] = newData
	}

	if _, err := h.client.CoreV1().ConfigMaps(h.namespace).Update(context.TODO(), cm, metav1.UpdateOptions{}); err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, nil)
}
