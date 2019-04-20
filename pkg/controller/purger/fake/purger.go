package fake

import (
	"context"

	"github.com/trinchan/saiki/pkg/store"
	"k8s.io/klog"
)

type FakePurge struct {
	s store.Storer
}

func New(s store.Storer) *FakePurge {
	f := &FakePurge{
		s: s,
	}
	return f
}

func (f *FakePurge) Queue(rev store.Revision) {
	if err := f.s.PurgeRevision(context.TODO(), rev); err != nil {
		klog.Fatalf("fake purge queue failed")
	}
}

func (f *FakePurge) Stop() {}
