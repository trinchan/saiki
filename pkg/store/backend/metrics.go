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

package backend

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	deleteLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Subsystem: "backend",
			Namespace: "saiki",
			Name:      "delete_latency",
			Help:      "Delete latency to the backend.",
		},
	)

	readDirLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Subsystem: "backend",
			Namespace: "saiki",
			Name:      "read_dir_latency",
			Help:      "Read directory latency to the backend.",
		},
	)

	walkLatency = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Subsystem: "backend",
			Namespace: "saiki",
			Name:      "walk_latency",
			Help:      "Walk latency to the backend.",
		},
	)
)

type instrumentedBackend struct {
	Backend
}

func NewInstrumentedBackend(b Backend) Backend {
	if br, ok := b.(BulkRemovingBackend); ok {
		return newInstrumentedBulkRemovingBackend(br)
	}
	return &instrumentedBackend{b}
}

type bulkRemovingInstrumentedBackend struct {
	Backend
	br BulkRemovingBackend
}

func newInstrumentedBulkRemovingBackend(br BulkRemovingBackend) *bulkRemovingInstrumentedBackend {
	return &bulkRemovingInstrumentedBackend{
		Backend: &instrumentedBackend{br},
		br:      br,
	}
}

func (b *bulkRemovingInstrumentedBackend) RemoveAll(ctx context.Context, keys []string) ([]string, error) {
	start := time.Now()
	deleted, err := b.br.RemoveAll(ctx, keys)
	deleteLatency.Observe(time.Since(start).Seconds())
	return deleted, err
}

func (b *instrumentedBackend) Remove(ctx context.Context, key string) error {
	start := time.Now()
	err := b.Backend.Remove(ctx, key)
	deleteLatency.Observe(time.Since(start).Seconds())
	return err
}

func (b *instrumentedBackend) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	start := time.Now()
	infos, err := b.Backend.ReadDir(ctx, path)
	readDirLatency.Observe(time.Since(start).Seconds())
	return infos, err
}

func (b *instrumentedBackend) Walk(ctx context.Context, root string, walkFn filepath.WalkFunc) error {
	start := time.Now()
	err := b.Backend.Walk(ctx, root, walkFn)
	walkLatency.Observe(time.Since(start).Seconds())
	return err
}

func init() {
	prometheus.MustRegister(
		readDirLatency,
		walkLatency,
		deleteLatency,
	)
}
