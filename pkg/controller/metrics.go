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

package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/client-go/util/workqueue"
)

type queueMetricsProvider struct{}

type noopMetric struct{}

func (noopMetric) Inc()            {}
func (noopMetric) Dec()            {}
func (noopMetric) Set(float64)     {}
func (noopMetric) Observe(float64) {}

func (p *queueMetricsProvider) NewDepthMetric(name string) workqueue.GaugeMetric {
	m := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "saiki",
		Subsystem: "controller",
		Name:      "workqueue_depth_" + name,
		Help:      "Depth of workqueue",
	})
	prometheus.MustRegister(m)
	return m
}

func (p *queueMetricsProvider) NewAddsMetric(name string) workqueue.CounterMetric {
	m := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "saiki",
		Subsystem: "controller",
		Name:      "workqueue_adds_" + name,
		Help:      "Total number of adds",
	})
	prometheus.MustRegister(m)
	return m
}

func (p *queueMetricsProvider) NewDeprecatedAddsMetric(name string) workqueue.CounterMetric {
	return noopMetric{}
}

func (p *queueMetricsProvider) NewDeprecatedDepthMetric(name string) workqueue.GaugeMetric {
	return noopMetric{}
}

func (p *queueMetricsProvider) NewDeprecatedLatencyMetric(name string) workqueue.SummaryMetric {
	return noopMetric{}
}

func (p *queueMetricsProvider) NewDeprecatedLongestRunningProcessorMetric(name string) workqueue.SummaryMetric {
	return noopMetric{}
}

func (p *queueMetricsProvider) NewDeprecatedLongestRunningProcessorMicrosecondsMetric(name string) workqueue.SettableGaugeMetric {
	return noopMetric{}
}

func (p *queueMetricsProvider) NewDeprecatedRetriesMetric(name string) workqueue.CounterMetric {
	return noopMetric{}
}

func (p *queueMetricsProvider) NewDeprecatedUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	return noopMetric{}
}

func (p *queueMetricsProvider) NewDeprecatedWorkDurationMetric(name string) workqueue.SummaryMetric {
	return noopMetric{}
}

func (p *queueMetricsProvider) NewLatencyMetric(name string) workqueue.HistogramMetric {
	m := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "saiki",
		Subsystem: "controller",
		Name:      "workqueue_latency_" + name,
		Help:      "Workqueue latency in microseconds",
	})
	prometheus.MustRegister(m)
	return m
}

func (p *queueMetricsProvider) NewLongestRunningProcessorSecondsMetric(name string) workqueue.SettableGaugeMetric {
	m := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "saiki",
		Subsystem: "controller",
		Name:      "longest_running_process_seconds_" + name,
		Help:      "Longest running processor (seconds)",
	})
	prometheus.MustRegister(m)
	return m
}

func (p *queueMetricsProvider) NewUnfinishedWorkSecondsMetric(name string) workqueue.SettableGaugeMetric {
	m := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "saiki",
		Subsystem: "controller",
		Name:      "workqueue_unfinished_work_seconds_" + name,
		Help:      "Unfinished work (seconds).",
	})
	prometheus.MustRegister(m)
	return m
}

func (p *queueMetricsProvider) NewWorkDurationMetric(name string) workqueue.HistogramMetric {
	m := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "saiki",
		Subsystem: "controller",
		Name:      "workqueue_duration_" + name,
		Help:      "Workqueue duration in microseconds",
	})
	prometheus.MustRegister(m)
	return m
}

func (p *queueMetricsProvider) NewRetriesMetric(name string) workqueue.CounterMetric {
	m := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "saiki",
		Subsystem: "controller",
		Name:      "workqueue_retries_" + name,
		Help:      "Total number of retries",
	})
	prometheus.MustRegister(m)
	return m
}

var (
	LastSeenResourceVersionGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: "controller",
			Namespace: "saiki",
			Name:      "last_seen_resource_version",
			Help:      "The resource version of the last item to be queued.",
		},
	)

	LastProcessedResourceVersionGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: "controller",
			Namespace: "saiki",
			Name:      "last_processed_resource_version",
			Help:      "The resource version of the last item to be processed.",
		},
	)

	ProcessingErrorsCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Subsystem: "controller",
			Namespace: "saiki",
			Name:      "processing_errors",
			Help:      "Count of how many errors while processing items.",
		},
	)
)

func init() {
	workqueue.SetProvider(&queueMetricsProvider{})
	prometheus.MustRegister(
		LastSeenResourceVersionGauge,
		LastProcessedResourceVersionGauge,
		ProcessingErrorsCounter,
	)
}
