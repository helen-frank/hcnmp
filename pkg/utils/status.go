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

package utils

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Status(u *unstructured.Unstructured) (string, error) {
	return status(u)
}

func StatusFromRuntime(o runtime.Object) (string, error) {
	u, err := RuntimeObjectToUnstructured(o)
	if err != nil {
		return StatusUnknown, err
	}
	return status(u)
}

func DeploymentsEventStatusFromRuntime(o runtime.Object) (string, error) {
	objOut, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
	if err != nil {
		return StatusUnknown, err
	}
	u := &unstructured.Unstructured{Object: objOut}
	if err != nil {
		return StatusUnknown, err
	}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	})
	return status(u)
}
