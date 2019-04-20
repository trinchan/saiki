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

	"github.com/trinchan/saiki/pkg/handler"
	"github.com/trinchan/saiki/pkg/store/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Storer interface {
	RevisionClient
	config.Configger
	// PurgeObject(tm metav1.TypeMeta, namespace, name string) error
	Sync(context.Context, handler.ResourceHandlers) (State, RevisionList, error)
	Close(context.Context) error
	Backend() string
}

type RevisionPurger interface {
	PurgeRevision(context.Context, Revision) error
	PurgeRevisions(context.Context, RevisionList) (RevisionList, error)
}

type PurgingRevisionGetter interface {
	RevisionGetter
	RevisionPurger
}

type RevisionClient interface {
	PurgingRevisionGetter
	Stater
}

type Stater interface {
	State(context.Context, StateOptions) (State, error)
}

type RevisionGetter interface {
	Revisions(namespace string) RevisionInterface
}

type RevisionInterface interface {
	Create(context.Context, Revision) (Revision, error)
	Update(context.Context, Revision) (Revision, error)
	Delete(context.Context, Revision) error
	Get(context.Context, Revision) (Revision, error)
	List(context.Context, metav1.TypeMeta, string, metav1.ListOptions) (RevisionList, error)
}

type AsyncResponse struct {
	Revision Revision
	Error    error
}

type AsyncWriteRevisionInterface interface {
	RevisionInterface
	AsyncCreate(context.Context, Revision) (<-chan AsyncResponse, error)
	AsyncUpdate(context.Context, Revision) (<-chan AsyncResponse, error)
	AsyncDelete(context.Context, Revision) (<-chan AsyncResponse, error)
}
