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
	"fmt"

	"github.com/trinchan/saiki/pkg/handler"
	"golang.org/x/sync/semaphore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog"
)

type ReadWriteStore struct {
	Storer
	reader         RevisionClient
	writeSemaphore *semaphore.Weighted
}

func NewReadWriteStorer(writer Storer, reader RevisionClient, concurrentWriteSemaphore *semaphore.Weighted) (Storer, error) {
	cs := &ReadWriteStore{
		Storer:         writer,
		reader:         reader,
		writeSemaphore: concurrentWriteSemaphore,
	}
	return cs, nil
}

func (s *ReadWriteStore) Backend() string {
	return fmt.Sprintf("%s-cached", s.Storer.Backend())
}

func (s *ReadWriteStore) Sync(ctx context.Context, handlers handler.ResourceHandlers) (State, RevisionList, error) {
	klog.V(2).Infof("syncing underlying store")
	state, toDelete, err := s.Storer.Sync(ctx, handlers)
	if err != nil {
		return nil, nil, err
	}
	klog.V(2).Infof("found %d objects in underlying store", len(state))
	klog.V(2).Infof("writing state to cache")
	for _, revisions := range state {
		for _, rev := range revisions {
			if _, err := s.reader.Revisions(rev.Namespace()).Create(ctx, rev); err != nil {
				return nil, nil, err
			}
		}
	}
	return state, toDelete, err
}

func (s *ReadWriteStore) PurgeRevision(ctx context.Context, rev Revision) error {
	err := s.Storer.PurgeRevision(ctx, rev)
	if err != nil {
		return err
	}
	return s.reader.PurgeRevision(ctx, rev)
}

func (s *ReadWriteStore) PurgeRevisions(ctx context.Context, revs RevisionList) (RevisionList, error) {
	revsDeletedFromWriteStore, writerErr := s.Storer.PurgeRevisions(ctx, revs)
	// right now this is okay since the only reader implementation using this
	// is the in memory reader which never errors, but ideally we combine the
	// errors... complicated.
	revsDeleted, _ := s.reader.PurgeRevisions(ctx, revsDeletedFromWriteStore)
	return revsDeleted, writerErr
}

func (s *ReadWriteStore) State(ctx context.Context, o StateOptions) (State, error) {
	return s.reader.State(ctx, o)
}

func (s *ReadWriteStore) Revisions(namespace string) RevisionInterface {
	r := &readwriteNamespaceStore{
		s:  s,
		ns: namespace,
	}
	if s.writeSemaphore != nil {
		asyncWriter := newAsyncRevisionNamespaceWriter(r, s.writeSemaphore)
		return NewInstrumentedAsyncRevisionInterface(asyncWriter)
	}
	return NewInstrumentedRevisionInterface(r)
}

type readwriteNamespaceStore struct {
	s  *ReadWriteStore
	ns string
}

func (s *readwriteNamespaceStore) Create(ctx context.Context, rev Revision) (Revision, error) {
	_, err := s.s.Storer.Revisions(s.ns).Create(ctx, rev)
	if err != nil {
		return nil, err
	}
	return s.s.reader.Revisions(s.ns).Create(ctx, rev)
}

func (s *readwriteNamespaceStore) Update(ctx context.Context, rev Revision) (Revision, error) {
	_, err := s.s.Storer.Revisions(s.ns).Update(ctx, rev)
	if err != nil {
		return nil, err
	}
	return s.s.reader.Revisions(s.ns).Update(ctx, rev)
}

func (s *readwriteNamespaceStore) Delete(ctx context.Context, rev Revision) error {
	err := s.s.Storer.Revisions(s.ns).Delete(ctx, rev)
	if err != nil {
		return err
	}
	return s.s.reader.Revisions(s.ns).Delete(ctx, rev)
}

func (s *readwriteNamespaceStore) Get(ctx context.Context, rev Revision) (Revision, error) {
	return s.s.reader.Revisions(s.ns).Get(ctx, rev)
}

func (s *readwriteNamespaceStore) List(ctx context.Context, tm metav1.TypeMeta, name string, opts metav1.ListOptions) (RevisionList, error) {
	return s.s.reader.Revisions(s.ns).List(ctx, tm, name, opts)
}
