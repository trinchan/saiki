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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	getCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: "store",
			Namespace: "saiki",
			Name:      "reads",
			Help:      "Number of get requests sent to the store.",
		},
	)

	getLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Subsystem: "store",
			Namespace: "saiki",
			Name:      "read_latency",
			Help:      "Get latency to the store.",
		},
	)

	createCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: "store",
			Namespace: "saiki",
			Name:      "creates",
			Help:      "Number of create requests to the store.",
		},
	)

	createLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Subsystem: "store",
			Namespace: "saiki",
			Name:      "create_latency",
			Help:      "Create latency to the store.",
		},
	)

	deleteCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: "store",
			Namespace: "saiki",
			Name:      "deletes",
			Help:      "Number of delete requests to the store.",
		},
	)

	deleteLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Subsystem: "store",
			Namespace: "saiki",
			Name:      "delete_latency",
			Help:      "Delete latency to the store.",
		},
	)

	listCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: "store",
			Namespace: "saiki",
			Name:      "lists",
			Help:      "Number of list requests sent to the store.",
		},
	)

	listLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Subsystem: "store",
			Namespace: "saiki",
			Name:      "list_latency",
			Help:      "List latency to the store.",
		},
	)
)

type instrumentedRevisionPurger struct {
	RevisionPurger
}

func NewInstrumentedRevisionPurger(r RevisionPurger) *instrumentedRevisionPurger {
	return &instrumentedRevisionPurger{r}
}

func (r *instrumentedRevisionPurger) PurgeRevision(ctx context.Context, rev Revision) error {
	deleteCounter.Inc()
	start := time.Now()
	err := r.RevisionPurger.PurgeRevision(ctx, rev)
	deleteLatency.Observe(time.Since(start).Seconds())
	return err
}

func (r *instrumentedRevisionPurger) PurgeRevisions(ctx context.Context, revs RevisionList) (RevisionList, error) {
	deleteCounter.Inc()
	start := time.Now()
	ret, err := r.RevisionPurger.PurgeRevisions(ctx, revs)
	deleteLatency.Observe(time.Since(start).Seconds())
	return ret, err
}

type instrumentedRevisionInterface struct {
	RevisionInterface
}

func NewInstrumentedRevisionInterface(r RevisionInterface) *instrumentedRevisionInterface {
	return &instrumentedRevisionInterface{r}
}

func (r *instrumentedRevisionInterface) Create(ctx context.Context, rev Revision) (Revision, error) {
	createCounter.Inc()
	start := time.Now()
	ret, err := r.RevisionInterface.Create(ctx, rev)
	createLatency.Observe(time.Since(start).Seconds())
	return ret, err
}

func (r *instrumentedRevisionInterface) Update(ctx context.Context, rev Revision) (Revision, error) {
	createCounter.Inc()
	start := time.Now()
	ret, err := r.RevisionInterface.Update(ctx, rev)
	createLatency.Observe(time.Since(start).Seconds())
	return ret, err
}

func (r *instrumentedRevisionInterface) Delete(ctx context.Context, rev Revision) error {
	deleteCounter.Inc()
	start := time.Now()
	err := r.RevisionInterface.Delete(ctx, rev)
	// Delete is really an update -- Purge is the real delete.
	createLatency.Observe(time.Since(start).Seconds())
	return err
}

func (r *instrumentedRevisionInterface) Get(ctx context.Context, rev Revision) (Revision, error) {
	getCounter.Inc()
	start := time.Now()
	ret, err := r.RevisionInterface.Get(ctx, rev)
	getLatency.Observe(time.Since(start).Seconds())
	return ret, err
}

func (r *instrumentedRevisionInterface) List(ctx context.Context, tm metav1.TypeMeta, name string, opts metav1.ListOptions) (RevisionList, error) {
	listCounter.Inc()
	start := time.Now()
	ret, err := r.RevisionInterface.List(ctx, tm, name, opts)
	listLatency.Observe(time.Since(start).Seconds())
	return ret, err
}

func NewInstrumentedAsyncRevisionInterface(r AsyncWriteRevisionInterface) *instrumentedAsyncRevisionInterface {
	return &instrumentedAsyncRevisionInterface{r}
}

type instrumentedAsyncRevisionInterface struct {
	AsyncWriteRevisionInterface
}

func (r *instrumentedAsyncRevisionInterface) Create(ctx context.Context, rev Revision) (Revision, error) {
	createCounter.Inc()
	start := time.Now()
	ret, err := r.AsyncWriteRevisionInterface.Create(ctx, rev)
	createLatency.Observe(time.Since(start).Seconds())
	return ret, err
}

func (r *instrumentedAsyncRevisionInterface) Update(ctx context.Context, rev Revision) (Revision, error) {
	createCounter.Inc()
	start := time.Now()
	ret, err := r.AsyncWriteRevisionInterface.Update(ctx, rev)
	createLatency.Observe(time.Since(start).Seconds())
	return ret, err
}

func (r *instrumentedAsyncRevisionInterface) Delete(ctx context.Context, rev Revision) error {
	deleteCounter.Inc()
	start := time.Now()
	err := r.AsyncWriteRevisionInterface.Delete(ctx, rev)
	// Delete is really an update -- Purge is the real delete.
	createLatency.Observe(time.Since(start).Seconds())
	return err
}

func (r *instrumentedAsyncRevisionInterface) Get(ctx context.Context, rev Revision) (Revision, error) {
	getCounter.Inc()
	start := time.Now()
	ret, err := r.AsyncWriteRevisionInterface.Get(ctx, rev)
	getLatency.Observe(time.Since(start).Seconds())
	return ret, err
}

func (r *instrumentedAsyncRevisionInterface) List(ctx context.Context, tm metav1.TypeMeta, name string, opts metav1.ListOptions) (RevisionList, error) {
	listCounter.Inc()
	start := time.Now()
	ret, err := r.AsyncWriteRevisionInterface.List(ctx, tm, name, opts)
	listLatency.Observe(time.Since(start).Seconds())
	return ret, err
}

func (r *instrumentedAsyncRevisionInterface) AsyncCreate(ctx context.Context, rev Revision) (<-chan AsyncResponse, error) {
	createCounter.Inc()
	start := time.Now()
	ret, err := r.AsyncWriteRevisionInterface.AsyncCreate(ctx, rev)
	createLatency.Observe(time.Since(start).Seconds())
	return ret, err
}

func (r *instrumentedAsyncRevisionInterface) AsyncUpdate(ctx context.Context, rev Revision) (<-chan AsyncResponse, error) {
	createCounter.Inc()
	start := time.Now()
	ret, err := r.AsyncWriteRevisionInterface.AsyncUpdate(ctx, rev)
	createLatency.Observe(time.Since(start).Seconds())
	return ret, err
}

func (r *instrumentedAsyncRevisionInterface) AsyncDelete(ctx context.Context, rev Revision) (<-chan AsyncResponse, error) {
	deleteCounter.Inc()
	start := time.Now()
	ret, err := r.AsyncWriteRevisionInterface.AsyncDelete(ctx, rev)
	// Delete is really an update -- Purge is the real delete.
	createLatency.Observe(time.Since(start).Seconds())
	return ret, err
}

func init() {
	prometheus.MustRegister(
		createCounter,
		createLatency,
		deleteCounter,
		deleteLatency,
		getCounter,
		getLatency,
		listCounter,
		listLatency,
	)
}
