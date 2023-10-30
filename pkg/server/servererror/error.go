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

package servererror

import (
	"github.com/gin-gonic/gin"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog"
)

func HandleError(c *gin.Context, code int, err error) {
	if c.IsAborted() || c.Writer.Size() > 0 {
		return
	}

	if err != nil {
		klog.Errorf("err: %+v", err)
	}

	switch t := err.(type) {
	case apierrors.APIStatus:
		c.JSON(int(t.Status().Code), err)
		return
	}
	response := map[string]string{}
	response["message"] = err.Error()
	c.JSON(code, response)
}
