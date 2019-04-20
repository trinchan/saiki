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

package reaper

import (
	"context"
	"time"

	"github.com/trinchan/saiki/pkg/controller/policy"
	"github.com/trinchan/saiki/pkg/controller/purger"
	"github.com/trinchan/saiki/pkg/store"
	"k8s.io/klog"
)

type Reaper interface {
	Stop()
}

type Reap struct {
	s            store.Storer
	policy       policy.BackupPolicy
	p            purger.Purger
	reapInterval *time.Ticker
	ctx          context.Context
}

func New(ctx context.Context, policy policy.BackupPolicy, s store.Storer, p purger.Purger, interval time.Duration) *Reap {
	reapInterval := time.NewTicker(interval)
	f := &Reap{
		s:            s,
		policy:       policy,
		p:            p,
		reapInterval: reapInterval,
		ctx:          ctx,
	}
	go func() {
		klog.Infof("starting reaper")
		// TODO wait group to wait for flush all on Close()?
		for range f.reapInterval.C {
			if err := f.reap(ctx); err != nil {
				reapErrorsCounter.Inc()
				klog.Infof("error reaping: %v", err)
			}
		}
	}()
	return f
}

func (r *Reap) Stop() {
	if r != nil && r.reapInterval != nil {
		r.reapInterval.Stop()
	}
}

func (r *Reap) reap(ctx context.Context) error {
	reapStart := time.Now()
	klog.Infof("reap cycle starting")
	state, err := r.s.State(ctx, store.StateOptions{
		AllNamespaces: true,
		AllRevisions:  true,
	})
	if err != nil {
		return err
	}
	var revsToReap store.RevisionList
	count := 0
	for _, revisions := range state {
		for i := range revisions {
			count++
			rev := revisions[i]
			// never purge the last revision of an object unless it's deleted
			if i == (len(revisions)-1) && !rev.Deleted() {
				continue
			}
			rule := r.policy.RuleFor(rev)
			if rule == nil {
				klog.Warningf("no rule defined for revision - will not reap: %s", rev)
				continue
			}
			reapThreshold := time.Now().Add(-rule.RPOPolicy.PurgeOlderThan.Duration())
			if reapThreshold.After(rev.RevisionTimestamp()) {
				revsToReap = append(revsToReap, rev)
			}
		}
	}
	defer func() {
		klog.Infof("reaper processed %d objects and queued %d objects for purge in %s", count, len(revsToReap), time.Since(reapStart))
	}()

	if len(revsToReap) == 0 {
		return nil
	}
	for _, rev := range revsToReap {
		reapCounter.Inc()
		r.p.Queue(rev)
	}
	return nil
}
