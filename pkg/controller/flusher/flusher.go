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

package flusher

import (
	"context"
	"sync"
	"time"

	"github.com/trinchan/saiki/pkg/controller/cache"
	"github.com/trinchan/saiki/pkg/controller/policy"
	"github.com/trinchan/saiki/pkg/store"
	"k8s.io/klog"
)

type Flusher interface {
	Queue(item store.Revision)
	Stop()
}

type Flush struct {
	s             store.Storer
	c             cache.RevisionCacher
	retryCache    cache.RevisionCacher
	rpo           *time.Ticker
	retryInterval *time.Ticker
	ctx           context.Context
}

func flushKeyFunc(rev store.Revision) string {
	return rev.ResourceKey()
}

func errorKeyFunc(rev store.Revision) string {
	return rev.Key()
}

func New(ctx context.Context, policy policy.BackupPolicy, s store.Storer) *Flush {
	rpo := time.NewTicker(policy.RPOPolicy.RPO.Duration())
	f := &Flush{
		s:             s,
		c:             cache.New(flushKeyFunc),
		retryCache:    cache.New(errorKeyFunc),
		rpo:           rpo,
		retryInterval: time.NewTicker(1 * time.Minute),
		ctx:           ctx,
	}
	go func() {
		klog.Infof("starting flusher")
		// TODO wait group to wait for flush all on Close()?
		for range f.rpo.C {
			if failedFlushRevs := f.flush(ctx, f.c); len(failedFlushRevs) > 0 {
				klog.Infof("starting flush cycle")
				for _, rev := range failedFlushRevs {
					flushErrorsCounter.Inc()
					klog.Infof("error flushing, will retry: %s", rev)
					f.retryCache.Cache(rev)
				}
			}
		}
	}()
	go func() {
		klog.Infof("starting retry flusher")
		for range f.retryInterval.C {
			klog.Infof("starting retry flush cycle")
			if failedFlushRevs := f.flush(ctx, f.retryCache); len(failedFlushRevs) > 0 {
				for _, rev := range failedFlushRevs {
					flushErrorsCounter.Inc()
					klog.Infof("error retrying flush, will retry: %s", rev)
					f.retryCache.Cache(rev)
				}
			}
		}
	}()
	return f
}

func (f *Flush) Queue(rev store.Revision) {
	klog.V(3).Infof("queueing for flush: %s", rev)
	f.c.Cache(rev)
}

func (f *Flush) Stop() {
	if f != nil && f.rpo != nil {
		f.rpo.Stop()
	}
	if f != nil && f.retryInterval != nil {
		f.retryInterval.Stop()
	}
}

func (f *Flush) flush(ctx context.Context, cache cache.RevisionCacher) store.RevisionList {
	toFlush := cache.SafeCopy()
	var failedFlushRevs store.RevisionList
	flushStart := time.Now()
	klog.Infof("flush starting [%d objects]", len(toFlush))
	defer func() { klog.Infof("flushed %d objects in %s", len(toFlush), time.Since(flushStart)) }()
	wg := &sync.WaitGroup{}
	for _, rev := range toFlush {
		flushCounter.Inc()
		klog.V(3).Infof("flushing rev: %s", rev)
		var err error
		client := f.s.Revisions(rev.Namespace())
		// do async if we can
		if asyncClient, ok := client.(store.AsyncWriteRevisionInterface); ok {
			var asyncResponse <-chan store.AsyncResponse
			if rev.Deleted() {
				asyncResponse, err = asyncClient.AsyncDelete(ctx, rev)
				wg.Add(1)
			} else {
				asyncResponse, err = asyncClient.AsyncUpdate(ctx, rev)
				wg.Add(1)
			}

			if err != nil {
				klog.Warningf("error flushing write of rev %s: %v", rev, err)
				failedFlushRevs = append(failedFlushRevs, rev)
				wg.Done()
				continue
			}

			go func(asyncResponse <-chan store.AsyncResponse, wg *sync.WaitGroup, rev store.Revision) {
				defer wg.Done()
				response := <-asyncResponse
				if response.Error != nil {
					klog.Warningf("error flushing write of rev %s: %v", rev, response.Error)
					failedFlushRevs = append(failedFlushRevs, rev)
				}
				f.c.Delete(rev)
			}(asyncResponse, wg, rev)
			lastFlushedResourceVersionGauge.Set(float64(rev.ResourceVersion()))
		} else {
			// fallback to non-async
			if rev.Deleted() {
				err = client.Delete(ctx, rev)
			} else {
				_, err = client.Update(ctx, rev)
			}
			klog.V(3).Infof("flushed rev: %s", rev)
			if err != nil {
				klog.Warningf("error flushing write of rev %s: %v", rev, err)
				failedFlushRevs = append(failedFlushRevs, rev)
			}
			f.c.Delete(rev)
			lastFlushedResourceVersionGauge.Set(float64(rev.ResourceVersion()))
		}
	}
	wg.Wait()
	return failedFlushRevs
}
