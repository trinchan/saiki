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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/trinchan/saiki/pkg/handler"
	"github.com/trinchan/saiki/pkg/store/backend"
	"github.com/trinchan/saiki/pkg/store/config"
	"github.com/trinchan/saiki/pkg/store/crypt"
	"golang.org/x/sync/semaphore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
)

type OptionFunc func(*RevisionStore)

type RevisionStore struct {
	config.Configger
	RevisionPurger
	backend     backend.Backend
	enableCache bool
	encrypter   crypt.Encrypter

	writeSemaphore *semaphore.Weighted

	latestCache map[string]string
	cacheLock   *sync.RWMutex

	deleteUntracked bool
}

func WithBackend(backend backend.Backend) OptionFunc {
	return func(s *RevisionStore) {
		s.backend = backend
	}
}

func WithEncrypter(e crypt.Encrypter) OptionFunc {
	return func(s *RevisionStore) {
		s.encrypter = e
	}
}

func WithCache(shouldCache bool) OptionFunc {
	return func(s *RevisionStore) {
		s.enableCache = shouldCache
	}
}

func WithConfigger(configger config.Configger) OptionFunc {
	return func(s *RevisionStore) {
		s.Configger = configger
	}
}

func WithConcurrentWrites(concurrentWrites int) OptionFunc {
	return func(s *RevisionStore) {
		s.writeSemaphore = semaphore.NewWeighted(int64(concurrentWrites))
	}
}

func WithDeleteUntracked(deleteUntracked bool) OptionFunc {
	return func(s *RevisionStore) {
		s.deleteUntracked = deleteUntracked
	}
}

func New(ctx context.Context, opts ...OptionFunc) (Storer, error) {
	s := &RevisionStore{
		latestCache: make(map[string]string),
		cacheLock:   &sync.RWMutex{},
	}
	for _, o := range opts {
		o(s)
	}
	if s.backend == nil {
		return nil, fmt.Errorf("backend cannot be empty")
	}
	if s.Configger == nil {
		s.Configger = config.NewInBackend(s.backend)
	}
	if s.RevisionPurger == nil {
		klog.V(2).Infof("setting instrumented purger")
		s.RevisionPurger = NewInstrumentedRevisionPurger(NewRevisionPurger(s))
	}

	klog.V(2).Infof("backend: %s", s.Backend())
	klog.V(2).Infof("using in-memory cache: %v", s.enableCache)
	if s.enableCache {
		return NewReadWriteStorer(s, NewInMemoryGetter(), s.writeSemaphore)
	}

	return s, nil
}

func NewFromEnv(ctx context.Context) (Storer, error) {
	encrypter, err := crypt.NewFromEnv()
	if err != nil {
		return nil, err
	}
	b, err := backend.NewFromEnv(ctx)
	if err != nil {
		return nil, err
	}
	enableCache := viper.GetBool("enable-cache")
	concurrentWrites := viper.GetInt("concurrent-writes")
	deleteUntracked := viper.GetBool("delete-untracked")
	return New(ctx, WithEncrypter(encrypter), WithBackend(b), WithCache(enableCache), WithConcurrentWrites(concurrentWrites), WithDeleteUntracked(deleteUntracked))
}

func (s *RevisionStore) Backend() string {
	return s.backend.Provider()
}

// func (s *Store) PurgeObject(tm metav1.TypeMeta, namespace, name string) error {
// 	revisions, err := s.Revisions(namespace).List(tm, name, metav1.ListOptions{})
// 	if err != nil {
// 		return err
// 	}
// 	for i := range revisions {
// 		err := s.backend.Remove(revisions[i].Key())
// 		if err != nil {
// 			return err
// 		}
// 	}
// 	return s.backend.Remove(dirForResource(tm.APIVersion, tm.Kind, namespace, name))
// }

func (s *RevisionStore) State(ctx context.Context, o StateOptions) (State, error) {
	res := make(map[string]RevisionList)
	var selector labels.Selector
	var err error
	if o.LabelSelector != nil {
		selector, err = metav1.LabelSelectorAsSelector(o.LabelSelector)
		if err != nil {
			return nil, err
		}
	}

	var root string
	switch {
	case o.APIVersion != "" && o.Kind != "" && (o.Namespace != "" && !o.AllNamespaces) && o.Name != "":
		root = ResourceKey(o.APIVersion, o.Kind, o.Namespace, o.Name)
	case o.APIVersion != "" && o.Kind != "" && (o.Namespace != "" && !o.AllNamespaces):
		root = KeyPrefixForAPIVersionKindNamespace(o.APIVersion, o.Kind, o.Namespace)
	case o.APIVersion != "" && o.Kind != "":
		root = KeyPrefixForAPIVersionKind(o.APIVersion, o.Kind)
	case o.APIVersion != "":
		root = KeyPrefixForAPIVersion(o.APIVersion)
	}
	klog.V(3).Infof("walk root: %s", root)
	err = s.backend.Walk(ctx, root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		dir := filepath.Dir(path)
		if dir == "" || dir == "." {
			return nil
		}

		key := strings.TrimPrefix(path, "/")

		rev, err := ParseLazyLoadingRevision(ctx, s, key)
		if err != nil {
			return err
		}
		// TODO filter this at the walk level by changing the root for speedups
		if !o.AllNamespaces && o.Namespace != "" && rev.Namespace() != o.Namespace {
			return filepath.SkipDir
		}

		if o.Name != "" && rev.Name() != o.Name {
			return filepath.SkipDir
		}

		if o.Kind != "" && rev.Kind() != o.Kind {
			return filepath.SkipDir
		}

		if o.APIVersion != "" && rev.APIVersion() != o.APIVersion {
			return filepath.SkipDir
		}

		if o.OnlyDeleted && !rev.Deleted() {
			return nil
		}

		if !o.Time.IsZero() && !o.Time.After(rev.RevisionTimestamp()) {
			return nil
		}

		resourceKey := rev.ResourceKey()

		if o.AllRevisions || res[resourceKey] == nil || rev.RevisionTimestamp().After(res[resourceKey][0].RevisionTimestamp()) {
			match := true
			if o.LabelSelector != nil {
				u, err := rev.Content()
				if err != nil {
					// TODO maybe group errors and return what we can instead of immediately erroring here
					return err
				}
				match = selector.Matches(labels.Set(u.GetLabels()))
			}

			if !match {
				return nil
			}

			if !o.AllRevisions {
				res[resourceKey] = RevisionList{rev}
				return nil
			}
			res[resourceKey] = append(res[resourceKey], rev)
		}
		return nil
	})
	if err != nil {
		klog.Warningf("error getting state: %v", err)
	}
	return res, err
}

func (s *RevisionStore) Sync(ctx context.Context, handlers handler.ResourceHandlers) (State, RevisionList, error) {
	state := make(map[string]RevisionList)
	var toDelete RevisionList
	err := s.backend.Walk(ctx, "", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		dir := filepath.Dir(path)
		if dir == "" || dir == "." {
			return nil
		}
		key := strings.TrimPrefix(path, "/")
		rev, err := ParseLazyLoadingRevision(ctx, s, key)
		if err != nil {
			return errors.Wrapf(err, "error parsing lazy revision from informer store: %s", key)
		}
		h := handlers.HandlerFor(rev.APIVersion(), rev.Kind(), rev.Namespace())
		if h == nil {
			klog.Warningf("configuration no longer tracks stored resource: %s", rev)
			if s.deleteUntracked {
				klog.Warningf("will delete untracked resource: %s", rev)
				toDelete = append(toDelete, rev)
				return nil
			} else {
				klog.Warningf("will skip untracked resource: %s", rev)
				return filepath.SkipDir
			}
		}

		state[rev.ResourceKey()] = append(state[rev.ResourceKey()], rev)
		// this is an implementation detail of storers... should probablyfigure out how to control this in our informers or
		// type ObjectMetaAccessor interface {
		// 	GetObjectMeta() Object
		// }
		// maybe implement this on Revisions to pose as an Unstructured when querying the store
		var storeKey string
		if len(rev.Namespace()) > 0 {
			storeKey = rev.Namespace() + "/"
		}
		storeKey += rev.Name()
		_, exists, err := h.Store.GetByKey(storeKey)
		if err != nil {
			return errors.Wrapf(err, "error getting revision from informer store: %s", rev)
		}
		if !exists {
			toDelete = append(toDelete, rev)
		}
		return nil
	})
	if err != nil {
		klog.Warningf("error syncing: %v", err)
	}
	return state, toDelete, err
}

func (s *RevisionStore) get(ctx context.Context, filepath string) (Revision, error) {
	f, err := s.backend.Open(ctx, filepath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var data []byte
	if s.encrypter != nil {
		data, err = s.encrypter.Decrypt(f)
	} else {
		data, err = ioutil.ReadAll(f)
	}
	if err != nil {
		return nil, err
	}
	storedU := new(unstructured.Unstructured)
	err = json.Unmarshal(data, storedU)
	if err != nil {
		return nil, err
	}
	return RevisionFrom(storedU), nil
}

func (s *RevisionStore) save(ctx context.Context, rev Revision) error {
	f, err := s.backend.Create(ctx, rev.Key())
	if err != nil {
		return err
	}
	defer f.Close()
	return s.write(f, rev)
}

func (s *RevisionStore) write(w io.Writer, rev Revision) error {
	u, err := rev.Content()
	if err != nil {
		return err
	}
	data, err := json.Marshal(u)
	if err != nil {
		return err
	}
	if s.encrypter != nil {
		data, err = s.encrypter.Encrypt(bytes.NewReader(data))
		if err != nil {
			return err
		}
	}
	_, err = w.Write(data)
	if err != nil {
		return err
	}
	return nil
}

func (s *RevisionStore) Close(ctx context.Context) error {
	return s.backend.Close(ctx)
}

func (s *RevisionStore) Revisions(namespace string) RevisionInterface {
	r := &revisionNamespaceStore{
		s:  s,
		ns: namespace,
	}
	if s.writeSemaphore != nil {
		asyncWriter := newAsyncRevisionNamespaceWriter(r, s.writeSemaphore)
		return NewInstrumentedAsyncRevisionInterface(asyncWriter)
	}
	return NewInstrumentedRevisionInterface(r)
}

type revisionNamespaceStore struct {
	s  *RevisionStore
	ns string
}

func (s *revisionNamespaceStore) Get(ctx context.Context, rev Revision) (Revision, error) {
	resourceKey := rev.ResourceKey()
	s.s.cacheLock.RLock()
	if latest, ok := s.s.latestCache[resourceKey]; ok {
		s.s.cacheLock.RUnlock()
		return s.s.get(ctx, RevisionKey(resourceKey, latest))
	}
	s.s.cacheLock.RUnlock()
	latestRevision, err := s.latestRevision(ctx, rev)
	if err != nil {
		return nil, err
	}
	if latestRevision == nil {
		return nil, nil
	}
	s.s.cacheLock.Lock()
	s.s.latestCache[resourceKey] = latestRevision.RevisionKey()
	s.s.cacheLock.Unlock()
	return s.s.get(ctx, RevisionKey(resourceKey, latestRevision.RevisionKey()))
}

func (s *revisionNamespaceStore) Create(ctx context.Context, rev Revision) (Revision, error) {
	return s.Update(ctx, rev)
}

func (s *revisionNamespaceStore) Update(ctx context.Context, rev Revision) (Revision, error) {
	err := s.s.save(ctx, rev)
	if err != nil {
		return nil, err
	}
	s.s.cacheLock.Lock()
	s.s.latestCache[rev.ResourceKey()] = rev.RevisionKey()
	s.s.cacheLock.Unlock()
	return rev, nil
}

func (s *revisionNamespaceStore) Delete(ctx context.Context, rev Revision) error {
	rev.SetDeleted(true)
	return s.s.save(ctx, rev)
}

func (s *revisionNamespaceStore) List(ctx context.Context, tm metav1.TypeMeta, name string, opts metav1.ListOptions) (RevisionList, error) {
	resourceKey := ResourceKey(tm.APIVersion, tm.Kind, s.ns, name)
	infos, err := s.s.backend.ReadDir(ctx, resourceKey)
	if err != nil {
		return nil, err
	}
	klog.V(5).Infof("read dir: %s - %+v", resourceKey, infos)
	var revisions RevisionList
	for i := range infos {
		r, err := ParseLazyLoadingRevision(ctx, s.s, RevisionKey(resourceKey, infos[i].Name()))
		if err != nil {
			return nil, err
		}
		if !r.Deleted() {
			revisions = append(revisions, r)
		}
	}
	sort.Sort(revisions)
	return revisions, nil
}

// func (s *namespacedStore) PurgeObject(tm metav1.TypeMeta, namespace, name string) error {
// 	return s.s.PurgeObject(tm, namespace, name)
// }

func (s *revisionNamespaceStore) latestRevision(ctx context.Context, rev Revision) (Revision, error) {
	revisions, err := s.s.Revisions(rev.Namespace()).List(ctx, metav1.TypeMeta{
		APIVersion: rev.APIVersion(),
		Kind:       rev.Kind(),
	}, rev.Name(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	latestTS := time.Time{}
	var latest Revision
	for i := range revisions {
		if revisions[i].RevisionTimestamp().After(latestTS) {
			latest = revisions[i]
		}
	}
	return latest, nil
}

type revisionPurger struct {
	s *RevisionStore
}

func NewRevisionPurger(s *RevisionStore) *revisionPurger {
	return &revisionPurger{s}
}

func (p *revisionPurger) PurgeRevision(ctx context.Context, rev Revision) error {
	keyToPurge := rev.Key()
	klog.V(2).Infof("purging key from store: %s", keyToPurge)
	return p.s.backend.Remove(ctx, keyToPurge)
}

func (p *revisionPurger) PurgeRevisions(ctx context.Context, revs RevisionList) (RevisionList, error) {
	if len(revs) == 0 {
		return nil, nil
	}
	keys := make([]string, len(revs))
	for i, rev := range revs {
		keys[i] = rev.Key()
	}

	var deletedRevs RevisionList

	if bulkRemovingBackend, ok := p.s.backend.(backend.BulkRemover); ok {
		klog.V(2).Infof("bulk purging backend")
		deletedKeys, err := bulkRemovingBackend.RemoveAll(ctx, keys)
		for _, deletedKey := range deletedKeys {
			rev, err := ParseLazyLoadingRevision(ctx, p.s, deletedKey)
			if err != nil {
				klog.Warningf("could not parse revision in bulk remove response: %s", deletedKey)
				continue
			}
			deletedRevs = append(deletedRevs, rev)
		}
		return deletedRevs, err
	}

	for _, key := range keys {
		klog.V(2).Infof("non-bulk purging backend")
		err := p.s.backend.Remove(ctx, key)
		if err != nil {
			klog.Warningf("error purging revision: %s - %v", key, err)
			continue
		}
		rev, err := ParseLazyLoadingRevision(ctx, p.s, key)
		if err != nil {
			klog.Warningf("could not parse revision: %s - %v", key, err)
			continue
		}
		deletedRevs = append(deletedRevs, rev)
	}
	return deletedRevs, nil
}
