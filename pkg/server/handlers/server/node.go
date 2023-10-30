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

package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/helen-frank/hcnmp/pkg/server/servererror"
	"github.com/helen-frank/hcnmp/pkg/zone/proxy"
)

// listNamespaceOfNode get all namespace on node
func (h *handler) listNamespaceOfNode(c *gin.Context) {
	name := c.Param("name")
	client, err := proxy.GetClusterPorxyClientFromCode(c.Param("clusterCode"))
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	podList, err := client.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + name,
	})
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	namespacesMap := make(map[string]struct{})
	for i := range podList.Items {
		namespacesMap[podList.Items[i].Namespace] = struct{}{}
	}

	namespaces := make([]string, 0, len(namespacesMap))
	for namespace := range namespacesMap {
		namespaces = append(namespaces, namespace)
	}

	c.JSON(http.StatusOK, namespaces)
}
