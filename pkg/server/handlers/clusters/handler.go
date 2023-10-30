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
	"github.com/helen-frank/hcnmp/pkg/zone/clientset"

	"github.com/gin-gonic/gin"
)

type handler struct {
	namespace         string
	clusterInfos      string
	localClusterInfos string
	client            clientset.Interface
}

func InstallHandlers(routerGroup *gin.RouterGroup, namespace, clusterInfos, localClusterInfos string, client clientset.Interface) {
	h := &handler{
		namespace:         namespace,
		clusterInfos:      clusterInfos,
		localClusterInfos: localClusterInfos,
		client:            client,
	}

	// /apis/cluster/v1/
	routerGroupV1 := routerGroup.Group("/v1")
	{
		routerGroupV1.POST("/code/:clusterCode", h.addCluster)
		routerGroupV1.DELETE("/code/:clusterCode", h.removeCluster)
		routerGroupV1.PUT("/code/:clusterCode", h.updateCluster)
		routerGroupV1.GET("/code/:clusterCode", h.getCluster)
		routerGroupV1.GET("/", h.getClusters)
		routerGroupV1.PATCH("/code/:clusterCode", h.applyCluster)
	}

}
