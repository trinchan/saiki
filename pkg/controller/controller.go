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
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"github.com/trinchan/saiki/pkg/controller/flusher"
	"github.com/trinchan/saiki/pkg/controller/policy"
	"github.com/trinchan/saiki/pkg/controller/purger"
	"github.com/trinchan/saiki/pkg/controller/reaper"
	"github.com/trinchan/saiki/pkg/discovery"
	"github.com/trinchan/saiki/pkg/handler"
	"github.com/trinchan/saiki/pkg/rpo"
	"github.com/trinchan/saiki/pkg/store"
	"github.com/trinchan/saiki/pkg/store/config"
	"github.com/trinchan/saiki/pkg/util/sliceutil"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
	kdiscovery "k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

type operator struct {
	kubeClient     kubernetes.Interface
	dynamicClient  dynamic.Interface
	dynamicFactory discovery.DynamicFactory

	resourceHandlers handler.ResourceHandlers
	store            store.Storer
	backupPolicy     policy.BackupPolicy
	flusher          flusher.Flusher
	purger           purger.Purger
	reaper           reaper.Reaper
	reapInterval     time.Duration

	metricsHandler   string
	readinessHandler string
	httpServerPort   int
	ready            bool

	threadiness int
	ctx         context.Context
}

const (
	DefaultReapInterval = time.Hour
)

type OptionFunc func(*operator)

func WithBackupPolicy(b policy.BackupPolicy) OptionFunc {
	return func(o *operator) {
		o.backupPolicy = b
	}
}

func WithStorer(s store.Storer) OptionFunc {
	return func(o *operator) {
		o.store = s
	}
}

func WithThreadiness(threadiness int) OptionFunc {
	return func(o *operator) {
		o.threadiness = threadiness
	}
}

func WithFlusher(flusher flusher.Flusher) OptionFunc {
	return func(o *operator) {
		o.flusher = flusher
	}
}

func WithPurger(purger purger.Purger) OptionFunc {
	return func(o *operator) {
		o.purger = purger
	}
}

func WithReaper(reaper reaper.Reaper) OptionFunc {
	return func(o *operator) {
		o.reaper = reaper
	}
}

func WithReapInterval(interval time.Duration) OptionFunc {
	return func(o *operator) {
		o.reapInterval = interval
	}
}

func WithMetricsHandler(path string) OptionFunc {
	return func(o *operator) {
		o.metricsHandler = path
	}
}

func WithReadinessHandler(path string) OptionFunc {
	return func(o *operator) {
		o.readinessHandler = path
	}
}

func WithHTTPServerPort(port int) OptionFunc {
	return func(o *operator) {
		o.httpServerPort = port
	}
}

func NewFromEnv(ctx context.Context, kubeClient kubernetes.Interface, dynamicClient dynamic.Interface) (*operator, error) {
	s, err := store.NewFromEnv(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "creating store")
	}
	b := new(policy.BackupPolicy)
	configFile := viper.GetString("config")
	f, err := os.Open(configFile)
	if err != nil {
		return nil, errors.Wrapf(err, "opening config file: %s", configFile)
	}
	err = yaml.NewYAMLOrJSONDecoder(f, 1024).Decode(b)
	if err != nil {
		return nil, errors.Wrapf(err, "parsing config file: %s", configFile)
	}
	return New(ctx, kubeClient, dynamicClient,
		WithBackupPolicy(*b),
		WithStorer(s),
		WithReapInterval(viper.GetDuration("reap-interval")),
		WithThreadiness(viper.GetInt("threadiness")),
		WithReadinessHandler(viper.GetString("readiness-handler")),
		WithMetricsHandler(viper.GetString("metrics-handler")),
		WithHTTPServerPort(viper.GetInt("port")),
	), nil
}

func New(ctx context.Context, kubeClient kubernetes.Interface, dynamicClient dynamic.Interface, options ...OptionFunc) *operator {
	o := &operator{
		kubeClient:       kubeClient,
		dynamicClient:    dynamicClient,
		dynamicFactory:   discovery.NewDynamicFactory(dynamicClient),
		resourceHandlers: make(handler.ResourceHandlers),
		threadiness:      4,
		reapInterval:     DefaultReapInterval,
		ctx:              ctx,
	}
	for _, f := range options {
		f(o)
	}
	if o.store == nil {
		klog.Fatalf("storer must be set")
	}
	if o.flusher == nil {
		o.flusher = flusher.New(ctx, o.backupPolicy, o.store)
	}
	if o.purger == nil {
		o.purger = purger.New(ctx, o.store, o.backupPolicy.RPOPolicy.RPO.Duration())
	}
	if o.reaper == nil {
		o.reaper = reaper.New(ctx, o.backupPolicy, o.store, o.purger, o.reapInterval)
	}
	_, apiResources, err := o.kubeClient.Discovery().ServerGroupsAndResources()
	if err != nil {
		klog.Fatalf(errors.Wrap(err, "discovering resources").Error())
	}

	conf := &config.Config{
		Resources: apiResources,
	}

	err = o.store.WriteConfig(o.ctx, conf)
	if err != nil {
		klog.Fatalf(errors.Wrap(err, "writing config").Error())
	}

	dynamics, err := o.getDynamics(apiResources)
	if err != nil {
		klog.Fatalf(errors.Wrap(err, "flattening desired api resources to dynamic clients").Error())
	}

	for _, dynamic := range dynamics {
		if err := o.watchDynamic(dynamic); err != nil {
			klog.Fatalf(errors.Wrap(err, "creating dynamic watches").Error())
		}
	}

	return o
}

func (o *operator) getDynamics(apiResources []*metav1.APIResourceList) ([]discovery.Dynamic, error) {
	requiredVerbs := kdiscovery.SupportsAllVerbs{Verbs: []string{"delete", "get", "list", "patch", "create", "update", "watch"}}
	dynamics := make(map[string][]discovery.Dynamic)
	for _, r := range apiResources {
		gv, err := schema.ParseGroupVersion(r.GroupVersion)
		if err != nil {
			return nil, err
		}
		for _, a := range r.APIResources {
			if !requiredVerbs.Match(r.GroupVersion, &a) {
				continue
			}
			for _, rule := range o.backupPolicy.Rules {
				if len(rule.APIGroups) == 0 || sliceutil.In(rule.APIGroups, r.GroupVersion) || rule.APIGroups[0] == "*" {
					if len(rule.Resources) == 0 || sliceutil.In(rule.Resources, a.Name) || rule.Resources[0] == "*" {
						if len(rule.Namespaces) == 0 || rule.Namespaces[0] == "*" {
							client, err := o.dynamicFactory.ClientForGroupVersionResource(gv, a, "")
							if err != nil {
								return nil, err
							}
							dynamics[gv.String()+"/"+a.Name] = []discovery.Dynamic{client}
							break
						} else {
							for _, namespace := range rule.Namespaces {
								client, err := o.dynamicFactory.ClientForGroupVersionResource(gv, a, namespace)
								if err != nil {
									return nil, err
								}
								dynamics[gv.String()+"/"+a.Name] = append(dynamics[gv.String()+"/"+a.Name], client)
							}
						}
					}
				}
			}
		}
	}

	var ret []discovery.Dynamic
	for _, clients := range dynamics {
		namespaces := make(map[string]bool)
		for _, client := range clients {
			if !namespaces[client.Namespace()] {
				klog.Infof("registering dynamic client for resource %s on namespace %q", client.GroupVersionResource(), client.Namespace())
				ret = append(ret, client)
				namespaces[client.Namespace()] = true
			}
		}
	}
	return ret, nil
}

type unstructuredPair struct {
	old *unstructured.Unstructured
	new *unstructured.Unstructured
}

func (o *operator) watchDynamic(client discovery.Dynamic) error {
	gvk := client.GroupVersionKind()
	lw := &cache.ListWatch{
		ListFunc:  client.List,
		WatchFunc: client.Watch,
	}
	queueName := fmt.Sprintf("%s_%s", gvk.Version, gvk.Kind)
	if gvk.Group != "" {
		queueName = fmt.Sprintf("%s_%s", gvk.Group, queueName)
	}
	if client.Namespace() != "" {
		queueName += fmt.Sprintf("_%s", client.Namespace())
	}
	queueName = strings.Replace(queueName, "-", "_", -1)
	queueName = strings.Replace(queueName, ".", "_", -1)
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), queueName)
	store, controller := cache.NewInformer(
		lw,
		&unstructured.Unstructured{},
		time.Hour,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				queue.Add(obj)
			},
			UpdateFunc: func(old interface{}, new interface{}) {
				queue.Add(unstructuredPair{
					old: old.(*unstructured.Unstructured),
					new: new.(*unstructured.Unstructured),
				})
			},
			DeleteFunc: func(obj interface{}) {
				queue.Add(obj)
			},
		},
	)

	o.resourceHandlers.Set(gvk.GroupVersion().String(), gvk.Kind, client.Namespace(), &handler.ResourceHandler{
		Controller: controller,
		Store:      store,
		Client:     client,
		Queue:      queue,
	})
	return nil
}

func (o *operator) syncStore() error {
	klog.Info("syncing store")
	storeSyncStart := time.Now()
	defer func() { klog.Infof("store synced in %s", time.Since(storeSyncStart)) }()
	state, revsToDelete, err := o.store.Sync(o.ctx, o.resourceHandlers)
	if err != nil {
		return err
	}
	klog.Infof("found %d objects in store", len(state))
	klog.Infof("found %d stale objects to delete", len(revsToDelete))
	for _, rev := range revsToDelete {
		rev.SetDeleted(true)
		o.flusher.Queue(rev)
	}
	return nil
}

func (o *operator) registerMetricsHandler() {
	if o.metricsHandler != "" {
		klog.Infof("registered Prometheus handler at: %s", o.metricsHandler)
		http.Handle(o.metricsHandler, promhttp.Handler())
	}
}

func (o *operator) registerReadinessHandler() {
	if o.readinessHandler != "" {
		klog.Infof("registered readiness check at: %s", o.readinessHandler)
		http.HandleFunc(o.readinessHandler, func(w http.ResponseWriter, r *http.Request) {
			if o.ready {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write([]byte("ok")); err != nil {
					klog.Warningf("error writing readiness response: %v", err)
				}
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
				if _, err := w.Write([]byte("saiki is still initializing")); err != nil {
					klog.Warningf("error writing readiness response: %v", err)
				}
			}
		})
	}
}

func (o *operator) startHTTPServer() {
	o.registerMetricsHandler()
	o.registerReadinessHandler()
	if o.httpServerPort != 0 {
		httpListenAddr := fmt.Sprintf(":%d", o.httpServerPort)
		klog.Infof("starting http handlers at: %s", httpListenAddr)
		if err := http.ListenAndServe(httpListenAddr, nil); err != nil {
			klog.Warningf("http server crashed: %v", err)
		}
	}
}

func (o *operator) Run() error {
	defer o.store.Close(o.ctx)
	defer o.flusher.Stop()
	defer o.purger.Stop()
	defer o.reaper.Stop()

	klog.V(4).Infof("starting saiki")
	go o.startHTTPServer()
	synced := make([]cache.InformerSynced, len(o.resourceHandlers))
	i := 0
	for resource, handler := range o.resourceHandlers {
		klog.Infof("starting controller for resource %s", resource)
		go handler.Controller.Run(o.ctx.Done())
		synced[i] = handler.Controller.HasSynced
		defer handler.Queue.ShutDown()
		i++
	}
	informerSyncStart := time.Now()
	klog.Info("waiting for informer caches to sync")
	if !cache.WaitForCacheSync(o.ctx.Done(), synced...) {
		return errors.New("failed to wait for caches to sync")
	}
	klog.Infof("informers synced in %s", time.Since(informerSyncStart))
	if err := o.syncStore(); err != nil {
		return errors.Wrap(err, "syncing store")
	}
	o.ready = true
	klog.Info("starting workers")

	for _, h := range o.resourceHandlers {
		for i := 0; i < o.threadiness; i++ {
			id := fmt.Sprintf("%s:%d", h.Client.GroupVersionResource(), i)
			klog.Infof("spinning up worker %s", id)
			go func(id string, queue workqueue.RateLimitingInterface, stopCh <-chan struct{}) {
				wait.Until(func() { o.runWorker(id, queue) }, time.Second, stopCh)
			}(id, h.Queue, o.ctx.Done())
		}
		go func(h *handler.ResourceHandler, stopCh <-chan struct{}) {
			for {
				select {
				case <-time.After(5 * time.Second):
					if h.Queue.Len() > 0 {
						klog.Infof("[%s] queue: %d, shutting down: %v", h.Client.GroupVersionResource(), h.Queue.Len(), h.Queue.ShuttingDown())
					}
				case <-stopCh:
					return
				}
			}
		}(h, o.ctx.Done())
	}
	<-o.ctx.Done()
	klog.V(4).Info("shutting down workers")
	return nil
}

func (o *operator) runWorker(id string, queue workqueue.RateLimitingInterface) {
	for o.processNextWorkItem(id, queue) {
	}
}

func different(old, new store.Revision, checkFields []string) bool {
	if old == nil {
		klog.V(5).Infof("old revision is nil, so this must be new: %s", new)
		return true
	}

	// short circuit without looking at content, same resource version == same object
	if old.ResourceVersion() == new.ResourceVersion() {
		return false
	}

	oldContent, err := old.Content()
	if err != nil {
		// assume it's different cuz we got an error... maybe
		// don't do this
		klog.Infof("error getting content from store for old revision: %s, assuming different", old)
		return true
	}

	// this should only be called with concrete revisions
	// i guess we could type assert
	newContent, err := new.Content()
	if err != nil {
		klog.Infof("error getting content from store for new revision: %s, assuming different ", new)
		return true
	}
	for i := range checkFields {
		klog.V(5).Infof("running check: %+v", checkFields[i])
		o, oldFound, err := unstructured.NestedFieldNoCopy(oldContent.Object, strings.Split(checkFields[i], ".")...)
		if err != nil {
			continue
		}
		n, newFound, err := unstructured.NestedFieldNoCopy(newContent.Object, strings.Split(checkFields[i], ".")...)
		if err != nil {
			continue
		}
		klog.V(5).Infof("old: %+v", o)
		klog.V(5).Infof("new: %+v", n)
		if oldFound != newFound {
			return true
		}
		if !reflect.DeepEqual(o, n) {
			klog.V(5).Infof("not equal")
			return true
		}
	}
	return false
}

// processNextWorkItem grabs work off the workqueue.
func (o *operator) processNextWorkItem(id string, queue workqueue.RateLimitingInterface) bool {
	obj, shutdown := queue.Get()
	if shutdown {
		klog.Infof("shutting down worker: %s", id)
		return false
	}
	defer queue.Done(obj)
	return o.processUnstructured(obj)
}

func (o *operator) processUnstructured(obj interface{}) bool {
	var old, u store.Revision
	switch item := obj.(type) {
	case cache.DeletedFinalStateUnknown:
		u = store.RevisionFrom(item.Obj.(*unstructured.Unstructured).DeepCopy())
	case *unstructured.Unstructured:
		u = store.RevisionFrom(item.DeepCopy())
	case unstructuredPair:
		old = store.RevisionFrom(item.old.DeepCopy())
		u = store.RevisionFrom(item.new.DeepCopy())
	}
	LastSeenResourceVersionGauge.Set(float64(u.ResourceVersion()))
	defer func() { LastProcessedResourceVersionGauge.Set(float64(u.ResourceVersion())) }()

	start := time.Now()
	defer func() { klog.V(3).Infof("processed in %s", time.Since(start)) }()

	rule := o.backupPolicy.RuleFor(u)
	if rule == nil {
		klog.V(3).Infof("no rule defined for resource: %s", u)
		return true
	}

	if o.handleResourceFilter(u, rule) {
		return true
	}

	handler := o.resourceHandlers.HandlerFor(u.APIVersion(), u.Kind(), u.Namespace())
	if handler == nil {
		klog.Warningf("handler not found for: %s", u)
		return true
	}

	if o.handleDeletion(handler, u) {
		return true
	}

	// try to use the old value from the watch, fallback to reading from the store
	if old == nil {
		var err error
		klog.V(4).Infof("old is nil, will fetch from store: %s", u)
		old, err = o.store.Revisions(u.Namespace()).Get(o.ctx, u)
		if old == nil || err != nil {
			klog.V(4).Infof("error getting stored item, will store new revision: %v", err)
		}
	}
	if !different(old, u, rule.Fields) {
		klog.V(3).Infof("discarding unchanged resource: %s", u)
		return true
	}

	klog.V(3).Infof("updating revision: %s", u)
	o.flusher.Queue(u)
	o.handleRotation(u, rule)
	return true
}

func (o *operator) handleResourceFilter(rev store.Revision, rule *policy.BackupRule) bool {
	if len(rule.ResourceNames) > 0 && !sliceutil.In(rule.ResourceNames, rev.Name()) {
		klog.V(3).Infof("discarding unmatched resource (name): %s", rev)
		return true
	}

	if rule.LabelSelector != nil {
		content, err := rev.Content()
		if err != nil {
			klog.Warningf("error getting content during resource filter, this should not happen: %s - %v", rev, err)
			return true
		}
		if !rule.LabelSelector.Matches(labels.Set(content.GetLabels())) {
			klog.V(3).Infof("discarding unmatched resource (labels): %s", rev)
			return true
		}
	}
	return false
}

func (o *operator) handleDeletion(handler *handler.ResourceHandler, rev store.Revision) bool {
	// these should be concrete revisions, no possible error
	u, _ := rev.Content()
	if _, exists, err := handler.Store.Get(u); !exists || err != nil {
		klog.V(3).Infof("deleting revision: %s", rev)
		rev.SetDeleted(true)
		o.flusher.Queue(rev)
		return true
	}
	return false
}

func (o *operator) handleRotation(rev store.Revision, rule *policy.BackupRule) {
	if rule.RPOPolicy != nil {
		// rotate must never say to purge all revisions of an object,
		// otherwise interactions with flushers become complicated.
		// flushers only try to write the latest revision, so that should be
		// okay... maybe. haven't thought it completely through.
		revsToPurge, err := rpo.Rotate(o.ctx, o.store, rev, *rule.RPOPolicy)
		if err != nil {
			klog.Warningf("error rotating %s - %v", rev, err)
			return
		}
		for _, rev := range revsToPurge {
			o.purger.Queue(rev)
		}
	}
}
