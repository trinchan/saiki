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

// cache is an implementation of
package cache

import (
	"sync"

	"github.com/trinchan/saiki/pkg/store"
)

type KeyFunc func(store.Revision) string

type RevisionCacher interface {
	SafeCopy() store.RevisionList
	Cache(rev store.Revision)
	Delete(rev store.Revision)
}

type Cache struct {
	m       map[string]store.Revision
	keyFunc KeyFunc
	lock    *sync.RWMutex
}

// New returns a new Cache with the given KeyFunc.
func New(keyFunc KeyFunc) *Cache {
	c := &Cache{
		m:       make(map[string]store.Revision),
		keyFunc: keyFunc,
		lock:    &sync.RWMutex{},
	}
	return c
}

func (c *Cache) Cache(rev store.Revision) {
	if rev == nil {
		return
	}
	key := c.keyFunc(rev)
	c.lock.Lock()
	defer c.lock.Unlock()
	cached := c.m[key]
	if cached == nil {
		c.m[key] = rev
		return
	}
	if cached.ResourceVersion() < rev.ResourceVersion() {
		c.m[key] = rev
		return
	}
}

func (c *Cache) SafeCopy() store.RevisionList {
	c.lock.RLock()
	copy := make(store.RevisionList, len(c.m))
	i := 0
	for _, obj := range c.m {
		copy[i] = obj
		i++
	}
	c.lock.RUnlock()
	return copy
}

func (c *Cache) Delete(rev store.Revision) {
	if rev == nil {
		return
	}
	key := c.keyFunc(rev)
	c.lock.Lock()
	cachedObj := c.m[key]
	if cachedObj != nil && cachedObj.ResourceVersion() == rev.ResourceVersion() {
		delete(c.m, key)
	}
	c.lock.Unlock()
}
