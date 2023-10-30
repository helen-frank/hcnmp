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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FilterPodsByControllerRef returns a subset of pods controlled by given controller resource, excluding deployments.
func FilterPodsByControllerRef(owner metav1.Object, remainingPods *[]corev1.Pod) []corev1.Pod {
	matchingPods := make([]corev1.Pod, 0)
	for podInx := 0; podInx < len(*remainingPods); podInx++ {
		if metav1.IsControlledBy(&(*remainingPods)[podInx], owner) {
			matchingPods = append(matchingPods, (*remainingPods)[podInx])
			(*remainingPods) = append((*remainingPods)[:podInx], (*remainingPods)[podInx+1:]...)
			podInx--
		}
	}
	return matchingPods
}
