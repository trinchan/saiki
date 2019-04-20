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

	"golang.org/x/sync/semaphore"
)

type asyncRevisionNamespaceWriter struct {
	RevisionInterface
	semaphore *semaphore.Weighted
}

func newAsyncRevisionNamespaceWriter(r RevisionInterface, semaphore *semaphore.Weighted) *asyncRevisionNamespaceWriter {
	return &asyncRevisionNamespaceWriter{
		RevisionInterface: r,
		semaphore:         semaphore,
	}
}

func (s *asyncRevisionNamespaceWriter) AsyncCreate(ctx context.Context, rev Revision) (<-chan AsyncResponse, error) {
	if err := s.semaphore.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	ret := make(chan AsyncResponse)
	go func() {
		defer s.semaphore.Release(1)
		defer close(ret)
		rev, err := s.Create(ctx, rev)
		ret <- AsyncResponse{rev, err}
	}()
	return ret, nil
}

func (s *asyncRevisionNamespaceWriter) AsyncUpdate(ctx context.Context, rev Revision) (<-chan AsyncResponse, error) {
	if err := s.semaphore.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	ret := make(chan AsyncResponse)
	go func() {
		defer s.semaphore.Release(1)
		defer close(ret)
		rev, err := s.Update(ctx, rev)
		ret <- AsyncResponse{rev, err}
	}()
	return ret, nil
}

func (s *asyncRevisionNamespaceWriter) AsyncDelete(ctx context.Context, rev Revision) (<-chan AsyncResponse, error) {
	ret := make(chan AsyncResponse)
	if err := s.semaphore.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	go func() {
		defer s.semaphore.Release(1)
		defer close(ret)
		err := s.Delete(ctx, rev)
		ret <- AsyncResponse{nil, err}
	}()
	return ret, nil
}
