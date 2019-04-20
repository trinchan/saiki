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

package purger

import (
	"context"
	"time"

	"github.com/trinchan/saiki/pkg/controller/cache"
	"github.com/trinchan/saiki/pkg/store"
	"k8s.io/klog"
)

type Purger interface {
	Queue(rev store.Revision)
	Stop()
}

func purgeKeyFunc(rev store.Revision) string {
	return rev.Key()
}

type Purge struct {
	s             store.Storer
	c             cache.RevisionCacher
	purgeInterval *time.Ticker
	ctx           context.Context
}

func New(ctx context.Context, s store.Storer, interval time.Duration) *Purge {
	purgeInterval := time.NewTicker(interval)
	f := &Purge{
		s:             s,
		c:             cache.New(purgeKeyFunc),
		purgeInterval: purgeInterval,
		ctx:           ctx,
	}
	go func() {
		klog.Infof("starting purger")
		// TODO wait group to wait for flush all on Close()?
		for range f.purgeInterval.C {
			f.purge(ctx)
		}
	}()
	return f
}

func (p *Purge) Queue(rev store.Revision) {
	klog.V(3).Infof("queueing for purge: %s", rev)
	p.c.Cache(rev)
}

func (p *Purge) Stop() {
	if p != nil && p.purgeInterval != nil {
		p.purgeInterval.Stop()
	}
}

func (p *Purge) purge(ctx context.Context) {
	toPurge := p.c.SafeCopy()
	purgeStart := time.Now()
	klog.Infof("purge starting [%d objects]", len(toPurge))
	defer func() { klog.Infof("purged %d objects in %s", len(toPurge), time.Since(purgeStart)) }()
	if len(toPurge) == 0 {
		return
	}
	revsToPurge := make(store.RevisionList, len(toPurge))
	i := 0
	for _, rev := range toPurge {
		revsToPurge[i] = rev
		i++
	}
	deletedRevs, err := p.s.PurgeRevisions(ctx, revsToPurge)
	if err != nil {
		klog.Warningf("error purging revisions: %v", err)
	}
	for _, obj := range deletedRevs {
		p.c.Delete(obj)
	}
}
