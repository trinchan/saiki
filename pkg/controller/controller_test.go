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

package controller

import (
	"context"
	"strconv"
	"testing"
	"time"

	flusherFake "github.com/trinchan/saiki/pkg/controller/flusher/fake"
	"github.com/trinchan/saiki/pkg/controller/policy"
	purgerFake "github.com/trinchan/saiki/pkg/controller/purger/fake"
	reaperFake "github.com/trinchan/saiki/pkg/controller/reaper/fake"
	"github.com/trinchan/saiki/pkg/rpo"
	"github.com/trinchan/saiki/pkg/store"
	"github.com/trinchan/saiki/pkg/store/backend/cloud/blob"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicFake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/klog"
)

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

func newController(ctx context.Context) *operator {
	client := fake.NewSimpleClientset(&corev1.Pod{})
	dynamic := dynamicFake.NewSimpleDynamicClient(runtime.NewScheme(), []runtime.Object{}...)
	backend, err := blob.New(ctx, blob.WithProvider(blob.MemProvider))
	if err != nil {
		klog.Fatal("failed to create memory backend")
	}
	s, err := store.New(ctx, store.WithBackend(backend))
	if err != nil {
		klog.Fatal("failed to create memory backend store")
	}
	b := policy.BackupPolicy{
		Rules: []policy.BackupRule{
			{
				APIGroups: []string{"*"},
			},
		},
		RPOPolicy: &rpo.RPOPolicy{
			PurgeOlderThan: rpo.Duration(0),
			RPO:            rpo.Duration(0),
		},
	}
	return New(ctx, client, dynamic,
		WithStorer(s),
		WithFlusher(flusherFake.New(s)),
		WithPurger(purgerFake.New(s)),
		WithReaper(reaperFake.New()),
		WithBackupPolicy(b))
}

func TestController(t *testing.T) {
	u := newUnstructured("v1", "Pod", "namespace", "name", 1)
	u2 := newUnstructured("v1", "Pod", "namespace", "name", 2)
	rev := store.RevisionFrom(u)
	rev2 := store.RevisionFrom(u2)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	c := newController(ctx)
	go func() {
		if err := c.Run(); err != nil {
			t.Errorf("unexpected error when stopping informer: %v", err)
		}
	}()
	c.processUnstructured(u)
	if _, err := c.store.Revisions(rev.Namespace()).Get(ctx, rev); err != nil {
		t.Errorf("unexpected error when fetching object: %v", err)
	}
	c.processUnstructured(u2)
	if _, err := c.store.Revisions(rev.Namespace()).Get(ctx, rev); err != nil {
		t.Errorf("unexpected error when fetching object: %v", err)
	}
	if _, err := c.store.Revisions(rev2.Namespace()).Get(ctx, rev); err != nil {
		t.Errorf("unexpected error when fetching object: %v", err)
	}
	<-ctx.Done()
	cancel()
}
