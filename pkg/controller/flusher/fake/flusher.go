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
	"context"

	"github.com/trinchan/saiki/pkg/store"
	"k8s.io/klog"
)

type FakeFlush struct {
	s store.Storer
}

func New(s store.Storer) *FakeFlush {
	f := &FakeFlush{
		s: s,
	}
	return f
}

func (f *FakeFlush) Queue(rev store.Revision) {
	if _, err := f.s.Revisions(rev.Namespace()).Update(context.TODO(), rev); err != nil {
		klog.Fatalf("fake flush queue failed")
	}
}

func (f *FakeFlush) Stop() {}
