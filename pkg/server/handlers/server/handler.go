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
	"github.com/gin-gonic/gin"

	"github.com/helen-frank/hcnmp/pkg/zone/clientset"
)

type handler struct {
	client clientset.Interface
}

func InstallHandlers(routerGroup *gin.RouterGroup, client clientset.Interface) {
	h := &handler{
		client: client,
	}

	// /apis/server/v1/
	routerGroupV1 := routerGroup.Group("/v1")
	{
		// Proxy cluster for all native api
		routerGroupV1.Any("/proxy/cluster/:clusterCode/*urlPath", h.proxyCluster)

		// node
		routerGroupV1.GET("/cluster/:clusterCode/node/:name/namespace", h.listNamespaceOfNode)

		// deployment
		routerGroupV1.GET("/cluster/:clusterCode/namespace/:namespace/deployments/:name/pods", h.listPodOfDeployment)
		routerGroupV1.POST("/cluster/:clusterCode/namespace/:namespace/deployments/:name/restart", h.restartDeployment)

		// pod
		routerGroupV1.GET("/cluster/:clusterCode/namespace/:namespace/pod/:name/connect", h.podNetConnectServer)
	}
}
