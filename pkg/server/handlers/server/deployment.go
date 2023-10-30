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
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"

	"github.com/helen-frank/hcnmp/pkg/server/servererror"
	"github.com/helen-frank/hcnmp/pkg/utils"
	"github.com/helen-frank/hcnmp/pkg/zone/proxy"
)

// listPodOfDeployment get all pod on deployment
func (h *handler) listPodOfDeployment(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	client, err := proxy.GetClusterPorxyClientFromCode(c.Param("clusterCode"))
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	deployment, err := client.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}
	resultList := corev1.PodList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "List",
			APIVersion: "v1",
		},
		Items: []corev1.Pod{},
	}
	if *deployment.Spec.Replicas != 0 {
		pods, err := utils.ListDeploymentPods(context.Background(), client, *deployment)
		if err != nil {
			servererror.HandleError(c, http.StatusInternalServerError, err)
			return
		}
		resultList = corev1.PodList{
			Items: pods,
		}
	}

	c.JSON(http.StatusOK, resultList)
}

func (h *handler) restartDeployment(c *gin.Context) {
	namespace := c.Param("namespace")
	name := c.Param("name")

	client, err := proxy.GetClusterPorxyClientFromCode(c.Param("clusterCode"))
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	deployClient := client.AppsV1().Deployments(namespace)

	oldDeploy, err := deployClient.Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	restartTime := time.Now().UnixNano()
	oldDeploy.Spec.Template.Labels["hcnmp.io/restart"] = strconv.FormatInt(restartTime, 10)

	deploy, err := deployClient.Update(context.Background(), oldDeploy, metav1.UpdateOptions{})
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	if deploy.ResourceVersion == oldDeploy.ResourceVersion {
		servererror.HandleError(c, http.StatusInternalServerError, errors.New("restart deployment failed"))
		return
	}

	op := metav1.SingleObject(deploy.ObjectMeta)
	timeout := int64(5 * 60) // 5 min
	op.TimeoutSeconds = &timeout
	op.ResourceVersion = deploy.ResourceVersion
	op.Watch = true

	ready := false
	wch, err := deployClient.Watch(context.Background(), op)
	if err != nil {
		servererror.HandleError(c, http.StatusInternalServerError, err)
		return
	}

	for event := range wch.ResultChan() {
		if status, err := utils.DeploymentsEventStatusFromRuntime(event.Object); err != nil {
			klog.Error(err)
			continue
		} else if ready = status == utils.StatusReady; ready {
			break
		}
	}

	if !ready {
		servererror.HandleError(c, http.StatusInternalServerError, errors.New("restart deployment failed"))
		return
	}

	c.JSON(200, nil)
}
