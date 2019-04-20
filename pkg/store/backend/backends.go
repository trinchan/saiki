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

package backend

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"github.com/trinchan/saiki/pkg/store/backend/cloud/blob"
	"k8s.io/klog"
)

var Providers = []string{
	"blob",
}

type Backend interface {
	Create(ctx context.Context, key string) (io.WriteCloser, error)
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Remove(ctx context.Context, key string) error
	ReadDir(ctx context.Context, path string) ([]os.FileInfo, error)
	Walk(ctx context.Context, root string, walkFn filepath.WalkFunc) error
	Close(ctx context.Context) error
	Provider() string
}

type BulkRemover interface {
	RemoveAll(ctx context.Context, keys []string) ([]string, error)
}

type BulkRemovingBackend interface {
	Backend
	BulkRemover
}

func NewFromEnv(ctx context.Context) (Backend, error) {
	var b Backend
	var err error
	switch viper.GetString("backend") {
	case "blob":
		klog.V(2).Infof("setting blob store from env")
		b, err = blob.NewFromEnv(ctx)
	default:
		return nil, fmt.Errorf("no backend store implementation found for %s", viper.GetString("backend"))
	}
	return NewInstrumentedBackend(b), err
}
