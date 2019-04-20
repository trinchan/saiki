/*
Copyright 2019 The saiki Authors

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
	"os"
	"os/signal"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/trinchan/saiki/pkg/build"
	"github.com/trinchan/saiki/pkg/controller"
	saikierr "github.com/trinchan/saiki/pkg/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"k8s.io/klog"
)

var (
	kubeCfg         *rest.Config
	kubeConfigFlags *genericclioptions.ConfigFlags
	dynamicClient   dynamic.Interface
	kubeClient      kubernetes.Interface
	ctx             context.Context
)

var watchCmd = &cobra.Command{
	Use:              "watch",
	Args:             cobra.NoArgs,
	Short:            "continuously backup a k8s cluster according to a backup policy",
	TraverseChildren: true,
	Run: func(cmd *cobra.Command, args []string) {
		c, err := controller.NewFromEnv(ctx, kubeClient, dynamicClient)
		if err != nil {
			klog.Fatalf("error setting up controller: %+v", err)
		}
		if err := c.Run(); err != nil {
			klog.Fatalf(errors.Wrap(err, "running controller").Error())
		}
	},
}

func init() {
	kubeConfigFlags = genericclioptions.NewConfigFlags(false).WithDeprecatedPasswordFlag()
	kubeConfigFlags.AddFlags(watchCmd.Flags())
	watchCmd.Flags().String("metrics-handler", "/metrics", "Path to serve Prometheus metrics.")
	watchCmd.Flags().String("readiness-handler", "/healthz", "Path to serve the readiness handler.")
	watchCmd.Flags().IntP("port", "p", 3800, "port for the http server (handles metrics and readiness checks)")
	watchCmd.Flags().StringP("config", "c", "config.yml", "The backup policy config file for use with the watcher.")
	watchCmd.Flags().Int("concurrent-writes", 25, "How many concurrent writes allowed to run in the watcher.")
	watchCmd.Flags().IntP("threadiness", "t", 4, "How many parallel workers to run in the watcher.")
	watchCmd.Flags().Duration("reap-interval", time.Hour, "How often to run the reap process to delete old revisions.")
	watchCmd.Flags().Bool("delete-untracked", false, "If set, delete all objects from the store that are not tracked by the current configuration. Be careful!")
	ctx = handleInterrupt(context.WithCancel(context.Background()))
	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(watchCmd)
}

func initConfig() {
	var err error
	if kubeCfg, err = kubeConfigFlags.ToRESTConfig(); err != nil {
		klog.Fatalf(errors.Wrap(err, "creating kubernetes rest config").Error())
	}
	kubeCfg.Wrap(transport.ContextCanceller(ctx, saikierr.ErrCanceled))
	kubeCfg.UserAgent = build.FullVersion()
	if dynamicClient, err = dynamic.NewForConfig(kubeCfg); err != nil {
		klog.Fatalf(errors.Wrap(err, "creating dynamic kubernetes client").Error())
	}
	if kubeClient, err = kubernetes.NewForConfig(kubeCfg); err != nil {
		klog.Fatalf(errors.Wrap(err, "creating default kubernetes client").Error())
	}
}

func handleInterrupt(ctx context.Context, cancel context.CancelFunc) context.Context {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx
}
