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

package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	applyconfigurationscorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/helen-frank/hcnmp/pkg/apis/cluster"
	"github.com/helen-frank/hcnmp/pkg/apis/config"
	"github.com/helen-frank/hcnmp/pkg/server"
	"github.com/helen-frank/hcnmp/pkg/utils"
	"github.com/helen-frank/hcnmp/pkg/zone"
	"github.com/helen-frank/hcnmp/pkg/zone/clientset"
	"github.com/helen-frank/hcnmp/pkg/zone/proxy"
)

type Options struct {
	CommandName string
	config      config.Config
	kubeclient  clientset.Interface
	genericclioptions.IOStreams
}

func NewCommand(name string, in io.Reader, out, errout io.Writer) *cobra.Command {
	o := NewOption(name, genericclioptions.IOStreams{In: in, Out: out, ErrOut: errout})
	cmd := &cobra.Command{
		Use: name,
		Long: templates.LongDesc(`
			hcnmp is kubernets Expansion
		`),
		Run: func(cmd *cobra.Command, _ []string) {
			cmdutil.CheckErr(o.Complete(cmd))
			cmdutil.CheckErr(o.Validate(cmd))
			cmdutil.CheckErr(o.Run(cmd))
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
			}
			return nil
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&o.config.Debug, "debug", true, "gin open DebugMode")
	flags.IntVar(&o.config.Port, "port", 8080, "hcnmp listen port")
	flags.StringVar(&o.config.KubeConfig, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")
	flags.StringVar(&o.config.NameSpace, "namespace", "hcnmp-system", "If present, the namespace scope for this CLI request")
	flags.StringVar(&o.config.ClusterInfos, "cluster-info", "hcnmp-cluster-info", "configmaps name used by hcnmp")
	flags.StringVar(&o.config.LocalClusterInfos, "local-cluster-info", "", "Local cluster-info")
	flags.StringVar(&o.config.BasicAuthUser, "basic-auth-user", "admin", "hcnmp basic auth user")
	flags.StringVar(&o.config.BasicAuthPassword, "basic-auth-password", "admin", "hcnmp basic auth password")
	return cmd
}

func (o *Options) Complete(cmd *cobra.Command) error {
	kubeconfig, err := clientcmd.BuildConfigFromFlags("", o.config.KubeConfig)
	if err != nil {
		return err
	}

	if o.kubeclient, err = clientset.NewForConfig(kubeconfig); err != nil {
		return err
	}

	return nil
}

func (o *Options) Validate(cmd *cobra.Command) error {
	cmExist := false
	if _, err := o.kubeclient.CoreV1().Namespaces().Get(cmd.Context(), o.config.NameSpace, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			if _, err = o.kubeclient.CoreV1().Namespaces().Create(cmd.Context(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: o.config.NameSpace}}, metav1.CreateOptions{}); err != nil {
				return err
			}

			if len(o.config.LocalClusterInfos) != 0 {
				if _, err := utils.CreateConfigMapsFromLocal(o.config.LocalClusterInfos, o.config.NameSpace, o.config.ClusterInfos, o.kubeclient); err != nil {
					return err
				}
				cmExist = true
			}
		} else {
			return err
		}
	}

	if !cmExist {
		if _, err := o.kubeclient.CoreV1().ConfigMaps(o.config.NameSpace).Get(cmd.Context(), o.config.ClusterInfos, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				if len(o.config.LocalClusterInfos) != 0 {
					if _, err := utils.CreateConfigMapsFromLocal(o.config.LocalClusterInfos, o.config.NameSpace, o.config.ClusterInfos, o.kubeclient); err != nil {
						return err
					}
				}
			} else {
				return err
			}
		} else {
			if len(o.config.LocalClusterInfos) != 0 {
				data, err := os.ReadFile(o.config.LocalClusterInfos)
				if err != nil {
					return err
				}
				clusterInfos := make([]*cluster.ClusterInfo, 0)
				if err = utils.Std2Jsoniter.Unmarshal(data, &clusterInfos); err != nil {
					return err
				}

				if len(clusterInfos) == 0 {
					klog.Warning("no proxy cluster")
				}

				binaryData := make(map[string][]byte, len(clusterInfos))
				for i := range clusterInfos {
					if clusterData, err := utils.Std2Jsoniter.Marshal(clusterInfos[i]); err != nil {
						return err
					} else {
						binaryData[clusterInfos[i].Code] = clusterData
					}
				}

				cm := applyconfigurationscorev1.ConfigMap(o.config.ClusterInfos, o.config.NameSpace)
				cm.BinaryData = binaryData

				if _, err := o.kubeclient.CoreV1().ConfigMaps(o.config.NameSpace).Apply(context.TODO(), cm, metav1.ApplyOptions{FieldManager: "application/apply-patch"}); err != nil {
					return err
				}
			}
		}
	}

	if len(o.config.BasicAuthUser) == 0 {
		return fmt.Errorf("basic-auth-user not empty")
	}

	if len(o.config.BasicAuthPassword) == 0 {
		return fmt.Errorf("basic-auth-password not empty")
	}

	return nil
}

func (o *Options) Run(cmd *cobra.Command) error {
	if err := proxy.InitProxy(o.config.ClusterInfos, o.config.NameSpace, o.config.LocalClusterInfos, o.kubeclient); err != nil {
		return err
	}

	zone.NameSpace = o.config.NameSpace

	if err := server.Run(&o.config, o.kubeclient); err != nil {
		klog.Errorf("failed to start server: %v", err)
		return err
	}
	return nil
}

func NewOption(name string, ioStream genericclioptions.IOStreams) *Options {
	return &Options{
		IOStreams:   ioStream,
		CommandName: name,
	}
}
