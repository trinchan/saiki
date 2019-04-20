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
