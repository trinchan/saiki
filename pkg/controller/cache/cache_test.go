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

package cache_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/trinchan/saiki/pkg/controller/cache"
	"github.com/trinchan/saiki/pkg/fake"
	"github.com/trinchan/saiki/pkg/store"
	"k8s.io/utils/diff"
)

// TODO test concurrency

func keyFunc(rev store.Revision) string {
	return rev.Key()
}

func TestCache(t *testing.T) {
	obj1 := fake.NewFakeRevision("v1-fake", "Pod-fake", "namespace-fake", "pod-fake", 1)
	obj2 := fake.NewFakeRevision("v1-fake", "Pod-fake", "namespace-fake", "pod-fake-2", 2)
	tcs := []struct {
		name string
		data store.RevisionList
		want store.RevisionList
	}{
		{
			name: "nil",
			data: nil,
			want: store.RevisionList{},
		},
		{
			name: "no objects",
			data: store.RevisionList{},
			want: store.RevisionList{},
		},
		{
			name: "one object",
			data: store.RevisionList{obj1},
			want: store.RevisionList{obj1},
		},
		{
			name: "one object",
			data: store.RevisionList{obj1},
			want: store.RevisionList{obj1},
		},
		{
			name: "two same objects",
			data: store.RevisionList{obj1, obj1},
			want: store.RevisionList{obj1},
		},
		{
			name: "two different objects",
			data: store.RevisionList{obj1, obj2},
			want: store.RevisionList{obj1, obj2},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c := cache.New(keyFunc)
			for _, obj := range tc.data {
				c.Cache(obj)
			}
			got := c.SafeCopy()
			sort.Sort(got)
			sort.Sort(tc.want)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Copy(%q) want: %v\ngot: %v\ndiff: %s", tc.name, tc.want, got, diff.ObjectReflectDiff(got, tc.want))
			}
		})
	}
}

func TestDelete(t *testing.T) {
	obj1 := fake.NewFakeRevision("v1-fake", "Pod-fake", "namespace-fake", "pod-fake", 1)
	obj2 := fake.NewFakeRevision("v1-fake", "Pod-fake", "namespace-fake", "pod-fake-2", 2)
	tcs := []struct {
		name   string
		data   store.RevisionList
		delete store.Revision
		want   store.RevisionList
	}{
		{
			name:   "nil",
			data:   nil,
			delete: nil,
			want:   store.RevisionList{},
		},
		{
			name:   "no objects",
			data:   store.RevisionList{},
			delete: nil,
			want:   store.RevisionList{},
		},
		{
			name:   "one object",
			data:   store.RevisionList{obj1},
			delete: obj1,
			want:   store.RevisionList{},
		},
		{
			name:   "two same objects",
			data:   store.RevisionList{obj1, obj1},
			delete: obj1,
			want:   store.RevisionList{},
		},
		{
			name:   "two different objects, delete first",
			data:   store.RevisionList{obj1, obj2},
			delete: obj1,
			want:   store.RevisionList{obj2},
		},
		{
			name:   "two different objects, delete second",
			data:   store.RevisionList{obj1, obj2},
			delete: obj2,
			want:   store.RevisionList{obj1},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			c := cache.New(keyFunc)
			for _, obj := range tc.data {
				c.Cache(obj)
			}
			c.Delete(tc.delete)
			got := c.SafeCopy()
			sort.Sort(got)
			sort.Sort(tc.want)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Delete(%q) want: %v\ngot: %v\ndiff: %s", tc.name, tc.want, got, diff.ObjectReflectDiff(got, tc.want))
			}
		})
	}
}
