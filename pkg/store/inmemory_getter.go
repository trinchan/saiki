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

package store

import (
	"context"
	"sort"
	"sync"

	saikierr "github.com/trinchan/saiki/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

type cache struct {
	m           map[string]map[int]Revision
	latestCache map[string]int
	lock        *sync.RWMutex
}

func (m *cache) Set(rev Revision) {
	m.lock.Lock()
	defer m.lock.Unlock()
	resource := rev.ResourceKey()
	if m.m[resource] == nil {
		m.m[resource] = make(map[int]Revision)
	}
	m.m[resource][rev.ResourceVersion()] = rev
	m.latestCache[resource] = rev.ResourceVersion()
}

func (m *cache) Get(rev Revision) Revision {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if revs, ok := m.m[rev.ResourceKey()]; ok {
		return revs[m.latestCache[rev.ResourceKey()]]
	}
	return nil
}

func (m *cache) Delete(rev Revision) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if revs, ok := m.m[rev.ResourceKey()]; ok {
		delete(revs, rev.ResourceVersion())
	}
}

func (m *cache) List(prefix string) RevisionList {
	m.lock.RLock()
	defer m.lock.RUnlock()
	var revisions RevisionList
	if revs, ok := m.m[prefix]; ok {
		for _, rev := range revs {
			revisions = append(revisions, rev)
		}
		return revisions
	}
	return nil
}

func (m *cache) State() State {
	m.lock.RLock()
	defer m.lock.RUnlock()
	ret := make(State)
	for resourceKey, revs := range m.m {
		for _, rev := range revs {
			ret[resourceKey] = append(ret[resourceKey], rev)
		}
	}
	return ret
}

type InMemoryRevisionGetter struct {
	cache *cache
}

func NewInMemoryGetter() *InMemoryRevisionGetter {
	klog.V(3).Infof("initialized in memory cache")
	cs := &InMemoryRevisionGetter{
		cache: &cache{
			m:           make(map[string]map[int]Revision),
			latestCache: make(map[string]int),
			lock:        &sync.RWMutex{},
		},
	}
	return cs
}

func (s *InMemoryRevisionGetter) Backend() string {
	return "inmemory"
}

func (s *InMemoryRevisionGetter) PurgeRevision(ctx context.Context, rev Revision) error {
	s.cache.Delete(rev)
	return nil
}

func (s *InMemoryRevisionGetter) PurgeRevisions(ctx context.Context, revs RevisionList) (RevisionList, error) {
	for _, rev := range revs {
		s.cache.Delete(rev)
	}
	return revs, nil
}

// TODO support label selectors for cached backend? not really needed right now
func (s *InMemoryRevisionGetter) State(ctx context.Context, o StateOptions) (State, error) {
	state := s.cache.State()
	for resourceKey, revs := range state {
		for i, rev := range revs {
			if o.APIVersion != "" && o.APIVersion != rev.APIVersion() {
				delete(state, resourceKey)
				break
			}
			if o.Kind != "" && o.Kind != rev.Kind() {
				delete(state, resourceKey)
				break
			}
			if o.Namespace != "" && o.Namespace != rev.Namespace() && !o.AllNamespaces {
				delete(state, resourceKey)
				break
			}
			if o.Name != "" && o.Name != rev.Name() {
				delete(state, resourceKey)
				break
			}
			if o.OnlyDeleted && !rev.Deleted() {
				state[resourceKey] = state[resourceKey].Cut(i, i+1)
				continue
			}
			if !o.Time.IsZero() && o.Time.After(rev.RevisionTimestamp()) {
				state[resourceKey] = state[resourceKey].Cut(i, i+1)
				continue
			}
		}
		if !o.AllRevisions {
			// only the last one
			state[resourceKey] = RevisionList{state[resourceKey][len(state[resourceKey])-1]}
		}
		sort.Sort(state[resourceKey])
	}
	return state, nil
}

func (s *InMemoryRevisionGetter) Revisions(namespace string) RevisionInterface {
	return &inmemoryRevisionNamespaceGetter{
		s:  s,
		ns: namespace,
	}
}

type inmemoryRevisionNamespaceGetter struct {
	s  *InMemoryRevisionGetter
	ns string
}

func (s *inmemoryRevisionNamespaceGetter) Create(ctx context.Context, rev Revision) (Revision, error) {
	return s.Update(ctx, rev)
}

func (s *inmemoryRevisionNamespaceGetter) Update(ctx context.Context, rev Revision) (Revision, error) {
	s.s.cache.Set(rev)
	return rev, nil
}

func (s *inmemoryRevisionNamespaceGetter) Delete(ctx context.Context, rev Revision) error {
	s.s.cache.Set(rev)
	return nil
}

func (s *inmemoryRevisionNamespaceGetter) Get(ctx context.Context, rev Revision) (Revision, error) {
	cached := s.s.cache.Get(rev)
	if cached == nil {
		return nil, saikierr.ErrNotFound
	}
	return cached, nil
}

func (s *inmemoryRevisionNamespaceGetter) List(ctx context.Context, tm metav1.TypeMeta, name string, opts metav1.ListOptions) (RevisionList, error) {
	revs := s.s.cache.List(ResourceKey(tm.APIVersion, tm.Kind, s.ns, name))
	if revs == nil {
		return nil, saikierr.ErrNotFound
	}
	return revs, nil
}
