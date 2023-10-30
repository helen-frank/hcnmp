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
	"net/http"

	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"

	"github.com/helen-frank/hcnmp/pkg/server/servererror"
	"github.com/helen-frank/hcnmp/pkg/zone/proxy"
)

func (h *handler) proxyCluster(c *gin.Context) {
	client, err := proxy.GetClusterPorxyClientFromCode(c.Param("clusterCode"))
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	request := client.RESTClient().Verb(c.Request.Method).AbsPath(c.Param("urlPath")).Body(c.Request.Body)
	for k, v := range c.Request.URL.Query() {
		var s string
		for i := range v {
			if i != 0 {
				s += ","
			}
			s += v[i]
		}
		request.Param(k, s)
	}
	for k, v := range c.Request.Header {
		request.SetHeader(k, v...)
	}

	var statusCode int
	data, err := request.Do(c).StatusCode(&statusCode).Raw()
	if err != nil {
		klog.Error(err)
		if serr, ok := err.(*apierrors.StatusError); ok {
			c.JSON(int(serr.ErrStatus.Code), err)
		} else {
			c.JSON(http.StatusInternalServerError, err)
		}
	}
	c.Data(statusCode, "application/json, text/plain, */*", data)
}
