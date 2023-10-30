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
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"github.com/helen-frank/hcnmp/pkg/zone/clientset"
)

// ListDeploymentPods returns a set of pods controlled by given deployment.
func ListDeploymentPods(oldCtx context.Context, client clientset.Interface, deployment appsv1.Deployment) ([]corev1.Pod, error) {
	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, err
	}

	var (
		allRS   *metav1.PartialObjectMetadataList
		allPods *v1.PodList
		ch      = make(chan error, 2)
	)

	go func() {
		allRS, err = client.Metadata().Resource(schema.GroupVersionResource{
			Group: "apps", Version: "v1", Resource: "replicasets"}).Namespace(deployment.Namespace).List(oldCtx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			ch <- err
			return
		}
		ch <- nil
	}()

	go func() {
		allPods, err = client.CoreV1().Pods(deployment.Namespace).List(oldCtx, metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			ch <- err
			return
		}
		ch <- nil
	}()

	var errs []error
	for range [2]struct{}{} {
		if err := <-ch; err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return nil, utilerrors.NewAggregate(errs)
	}

	if len(allRS.Items) == 0 || len(allPods.Items) == 0 {
		return nil, nil
	}

	return FilterDeploymentPodsByOwnerReference(deployment, allRS.Items, allPods.Items), nil
}

// FilterDeploymentPodsByOwnerReference returns a subset of pods controlled by given deployment.
func FilterDeploymentPodsByOwnerReference(deployment appsv1.Deployment, allRS []metav1.PartialObjectMetadata,
	allPods []corev1.Pod) []corev1.Pod {
	matchingPods := make([]corev1.Pod, 0)
	for rsk := range allRS {
		if len(allPods) == 0 {
			break
		}
		if metav1.IsControlledBy(&allRS[rsk], &deployment) {
			matchingPods = append(matchingPods, FilterPodsByControllerRef(&allRS[rsk], &allPods)...)
		}
	}
	return matchingPods
}
