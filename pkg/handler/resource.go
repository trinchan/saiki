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

package handler

import (
	"path/filepath"

	"github.com/trinchan/saiki/pkg/discovery"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type ResourceHandlers map[string]*ResourceHandler

type ResourceHandler struct {
	Controller cache.Controller
	Store      cache.Store
	Queue      workqueue.RateLimitingInterface
	Client     discovery.Dynamic
}

func key(apiVersion, kind, namespace string) string {
	return filepath.Join(apiVersion, kind, namespace)
}

func (r ResourceHandlers) Set(apiVersion, kind, namespace string, h *ResourceHandler) {
	key := key(apiVersion, kind, namespace)
	r[key] = h
}

func (r ResourceHandlers) HandlerFor(apiVersion, kind, namespace string) *ResourceHandler {
	nsKey := key(apiVersion, kind, namespace)
	h, ok := r[nsKey]
	if !ok {
		clusterScopedKey := key(apiVersion, kind, "")
		h = r[clusterScopedKey]
	}
	return h
}
