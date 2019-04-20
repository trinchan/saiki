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
	"github.com/prometheus/client_golang/prometheus"
)

var (
	flushCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: "flusher",
			Namespace: "saiki",
			Name:      "count",
			Help:      "Count of how many items flushed.",
		},
	)

	flushErrorsCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: "flusher",
			Namespace: "saiki",
			Name:      "errors",
			Help:      "Count of how many errors while flushing.",
		},
	)

	lastFlushedResourceVersionGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: "flusher",
			Namespace: "saiki",
			Name:      "last_flushed_resource_version",
			Help:      "The resource version of the last item to be flushed.",
		},
	)
)

func init() {
	prometheus.MustRegister(
		flushErrorsCounter,
		flushCounter,
		lastFlushedResourceVersionGauge,
	)
}
