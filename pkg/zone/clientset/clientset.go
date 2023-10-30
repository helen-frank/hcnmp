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

package clientset

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog"
)

var (
	overlyCautiousIllegalFileCharacters = regexp.MustCompile(`[^(\w/\.)]`)
	// make sure that a Clientset instance implement the interface.
	_ = Interface(&Clientset{})
)

type Interface interface {
	kubernetes.Interface
	Metadata() metadata.Interface
	dynamic.Interface
	ClientConfig() *rest.Config
	CachedDiscovery() discovery.CachedDiscoveryInterface
}

type Clientset struct {
	config *rest.Config
	*kubernetes.Clientset
	dynamicClient         dynamic.Interface
	ctx                   context.Context
	metadata              metadata.Interface
	CachedDiscoveryClient discovery.CachedDiscoveryInterface
}

// Metadata return metadata client
func (c *Clientset) Metadata() metadata.Interface {
	return c.metadata
}

// Resource implement kuberentes dynamic interface
func (c *Clientset) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return c.dynamicClient.Resource(resource)
}

// ClientConfig returns a complete client config
func (c *Clientset) ClientConfig() *rest.Config {
	if c == nil {
		return nil
	}
	return c.config
}

// CachedDiscovery returns a complete CachedDiscovery config
func (c *Clientset) CachedDiscovery() discovery.CachedDiscoveryInterface {
	return c.CachedDiscoveryClient
}

// NewForConfig creates a new Clientset for the given config.
func NewForConfig(c *rest.Config) (*Clientset, error) {
	var sc Clientset
	var err error

	sc.config = c
	sc.ctx = context.Background()

	sc.Clientset, err = kubernetes.NewForConfig(c)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	sc.metadata, err = metadata.NewForConfig(c)
	if err != nil {
		klog.Error(err)
		return nil, err
	}
	sc.dynamicClient, err = dynamic.NewForConfig(c)
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	discoveryCacheDir := computeDiscoverCacheDir(filepath.Join(homedir.HomeDir(), ".kube", "cache", "discovery"), c.Host)
	httpCacheDir := filepath.Join(homedir.HomeDir(), ".kube", "http-cache")
	sc.CachedDiscoveryClient, err = disk.NewCachedDiscoveryClientForConfig(c, discoveryCacheDir, httpCacheDir, time.Duration(10*time.Minute))
	if err != nil {
		klog.Error(err)
		return nil, err
	}

	return &sc, nil
}

// computeDiscoverCacheDir takes the parentDir and the host and comes up with a "usually non-colliding" name.
func computeDiscoverCacheDir(parentDir, host string) string {
	// strip the optional scheme from host if its there:
	schemelessHost := strings.Replace(strings.Replace(host, "https://", "", 1), "http://", "", 1)
	// now do a simple collapse of non-AZ09 characters.  Collisions are possible but unlikely.  Even if we do collide the problem is short lived
	safeHost := overlyCautiousIllegalFileCharacters.ReplaceAllString(schemelessHost, "_")
	return filepath.Join(parentDir, safeHost)
}
