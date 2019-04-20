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
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/trinchan/saiki/pkg/store/backend"
	"github.com/trinchan/saiki/pkg/store/backend/cloud/blob"
	"github.com/trinchan/saiki/pkg/store/crypt"
	"k8s.io/klog"
)

var rootCmd = &cobra.Command{
	Use: "saiki",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if err := viper.BindPFlags(cmd.Flags()); err != nil {
			klog.Fatalf("error binding flags to viper: %v", err)
		}
	},
	Short: "continuous backup of k8s resources",
	Long:  ``,
}

func init() {
	rootCmd.PersistentFlags().StringP("encryption-key", "k", "", "encryption key for use with the store")
	rootCmd.PersistentFlags().StringP("encryption-algorithm", "a", "AES", fmt.Sprintf("encryption algorithm for use with the store [%s]", strings.Join(crypt.Providers, "|")))
	rootCmd.PersistentFlags().StringP("backend", "b", "blob", fmt.Sprintf("backend store to use [%s]", strings.Join(backend.Providers, "|")))
	rootCmd.PersistentFlags().String("blob-provider", "filesystem", fmt.Sprintf("blob storage provider for use with the blob backend [%s]", strings.Join(blob.Providers, "|")))
	rootCmd.PersistentFlags().String("aws-bucket", "", "bucket name for use with the S3 blob backend")
	rootCmd.PersistentFlags().String("aws-endpoint", "", "endpoint for use for when using the S3 blob backend")
	rootCmd.PersistentFlags().String("aws-default-region", "", "region for use with the S3 blob backend")
	rootCmd.PersistentFlags().String("aws-access-key-id", "", "access key for use with the S3 blob backend")
	rootCmd.PersistentFlags().String("aws-secret-access-key", "", "secret access key for use with the S3 blob backend")
	rootCmd.PersistentFlags().String("aws-session-token", "", "session token for use with the S3 blob backend")
	rootCmd.PersistentFlags().String("root", "", "If set, the root directory for use with with filesystem backend providers")
	rootCmd.PersistentFlags().Bool("enable-cache", true, "keep in-memory cache of backend state (highly recommended)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
