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

import "github.com/prometheus/client_golang/prometheus"

var (
	reapCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: "reaper",
			Namespace: "saiki",
			Name:      "count",
			Help:      "Count of how many items reaped.",
		},
	)

	reapErrorsCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: "reaper",
			Namespace: "saiki",
			Name:      "errors",
			Help:      "Count of how many errors while reaping.",
		},
	)
)

func init() {
	prometheus.MustRegister(
		reapErrorsCounter,
		reapCounter,
	)
}
