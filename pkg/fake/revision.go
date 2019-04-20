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

package fake

import (
	"strconv"

	"github.com/trinchan/saiki/pkg/store"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type FakeRevision struct {
	*store.ConcreteRevision
}

func newUnstructured(apiVersion, kind, namespace, name string, resourceVersion int) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace":       namespace,
				"name":            name,
				"resourceVersion": strconv.Itoa(resourceVersion),
			},
			"spec": name,
		},
	}
}

func NewFakeRevision(apiVersion, kind, namespace, name string, resourceVersion int) store.Revision {
	u := newUnstructured(apiVersion, kind, namespace, name, resourceVersion)
	return FakeRevision{store.RevisionFrom(u)}
}
