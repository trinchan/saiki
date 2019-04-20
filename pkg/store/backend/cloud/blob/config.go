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

package blob

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/spf13/viper"
	"gocloud.dev/blob/fileblob"
	"gocloud.dev/blob/memblob"
)

var (
	DefaultFSRoot    = ".saiki"
	DefaultCloudRoot = ""
)

type Config struct {
	BucketName string
	Root       string
	S3         *aws.Config
	Mem        *memblob.Options
	FS         *fileblob.Options
}

func ConfigFromEnv(provider Provider) Config {
	bucket := viper.GetString("aws-bucket")
	root := viper.GetString("root")
	if root == "" {
		switch provider {
		case S3Provider, MemProvider:
			root = DefaultCloudRoot
		default:
			root = DefaultFSRoot
		}
	}
	return Config{
		BucketName: bucket,
		Root:       root,
		S3:         s3ConfigFromEnv(),
		Mem:        &memblob.Options{},
		FS:         &fileblob.Options{},
	}
}

func s3ConfigFromEnv() *aws.Config {
	endpoint := viper.GetString("aws-endpoint")
	region := viper.GetString("aws-default-region")
	accessKeyID := viper.GetString("aws-access-key-id")
	secretAccessKey := viper.GetString("aws-secret-access-key")
	sessionToken := viper.GetString("aws-session-token")

	return aws.NewConfig().WithEndpoint(endpoint).WithRegion(region).WithCredentials(credentials.NewStaticCredentials(accessKeyID, secretAccessKey, sessionToken))
}
