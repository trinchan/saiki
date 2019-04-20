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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"
)

type Revision interface {
	json.Marshaler
	fmt.Stringer
	Content() (*unstructured.Unstructured, error)
	Namespace() string
	Name() string
	ResourceVersion() int
	RevisionTimestamp() time.Time
	APIVersion() string
	Kind() string
	RevisionKey() string
	ResourceKey() string
	Key() string
	SetDeleted(bool)
	Deleted() bool
	TypeMeta() metav1.TypeMeta
}

type revision struct {
	apiVersion        string
	kind              string
	namespace         string
	name              string
	resourceVersion   int
	revisionTimestamp time.Time
	deleted           bool
}

type ConcreteRevision struct {
	revision
	content *unstructured.Unstructured
}

func (c *ConcreteRevision) APIVersion() string { return c.apiVersion }
func (c *ConcreteRevision) Kind() string       { return c.kind }
func (c *ConcreteRevision) TypeMeta() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: c.APIVersion(), Kind: c.Kind(),
	}
}
func (c *ConcreteRevision) Namespace() string                            { return c.namespace }
func (c *ConcreteRevision) Name() string                                 { return c.name }
func (c *ConcreteRevision) ResourceVersion() int                         { return c.resourceVersion }
func (c *ConcreteRevision) RevisionTimestamp() time.Time                 { return c.revisionTimestamp }
func (c *ConcreteRevision) Deleted() bool                                { return c.deleted }
func (c *ConcreteRevision) SetDeleted(deleted bool)                      { c.deleted = deleted }
func (c *ConcreteRevision) Content() (*unstructured.Unstructured, error) { return c.content, nil }
func (c *ConcreteRevision) String() string {
	return fmt.Sprintf("%s/%s/%s/%s/%d/%v", c.APIVersion(), c.Kind(), c.Namespace(), c.Name(), c.ResourceVersion(), c.Deleted())
}
func (c *ConcreteRevision) MarshalJSON() ([]byte, error) {
	content, err := c.Content()
	if err != nil {
		return nil, err
	}
	out := struct {
		APIVersion        string                 `json:"apiVersion"`
		Kind              string                 `json:"kind"`
		Namespace         string                 `json:"namespace"`
		Name              string                 `json:"name"`
		ResourceVersion   int                    `json:"resourceVersion"`
		RevisionTimestamp int64                  `json:"revisionTimestamp"`
		Deleted           bool                   `json:"deleted"`
		Content           map[string]interface{} `json:"content"`
	}{
		APIVersion:        c.APIVersion(),
		Kind:              c.Kind(),
		Namespace:         c.Namespace(),
		Name:              c.Name(),
		ResourceVersion:   c.ResourceVersion(),
		RevisionTimestamp: c.RevisionTimestamp().Unix(),
		Deleted:           c.Deleted(),
		Content:           content.Object,
	}
	return json.Marshal(out)
}

type LazyLoadingRevision struct {
	revision
	once    *sync.Once
	content *unstructured.Unstructured
	s       Storer
}

func (l *LazyLoadingRevision) APIVersion() string { return l.apiVersion }
func (l *LazyLoadingRevision) Kind() string       { return l.kind }
func (l *LazyLoadingRevision) TypeMeta() metav1.TypeMeta {
	return metav1.TypeMeta{Kind: l.kind, APIVersion: l.apiVersion}
}
func (l *LazyLoadingRevision) Namespace() string            { return l.namespace }
func (l *LazyLoadingRevision) Name() string                 { return l.name }
func (l *LazyLoadingRevision) ResourceVersion() int         { return l.resourceVersion }
func (l *LazyLoadingRevision) RevisionTimestamp() time.Time { return l.revisionTimestamp }
func (l *LazyLoadingRevision) SetDeleted(deleted bool)      { l.deleted = deleted }
func (l *LazyLoadingRevision) Deleted() bool                { return l.deleted }
func (l *LazyLoadingRevision) String() string {
	return fmt.Sprintf("%s/%s/%s/%s/%d/%v", l.APIVersion(), l.Kind(), l.Namespace(), l.Name(), l.ResourceVersion(), l.Deleted())
}
func (l *LazyLoadingRevision) MarshalJSON() ([]byte, error) {
	out := struct {
		APIVersion        string `json:"apiVersion"`
		Kind              string `json:"kind"`
		Namespace         string `json:"namespace"`
		Name              string `json:"name"`
		ResourceVersion   int    `json:"resourceVersion"`
		RevisionTimestamp int64  `json:"revisionTimestamp"`
		Deleted           bool   `json:"deleted"`
	}{
		APIVersion:        l.APIVersion(),
		Kind:              l.Kind(),
		Namespace:         l.Namespace(),
		Name:              l.Name(),
		ResourceVersion:   l.ResourceVersion(),
		RevisionTimestamp: l.RevisionTimestamp().Unix(),
		Deleted:           l.Deleted(),
	}
	return json.Marshal(out)
}

func (l *LazyLoadingRevision) Content() (*unstructured.Unstructured, error) {
	// err outside scope so we can retunr it
	var err error
	l.once.Do(func() {
		var rev Revision
		rev, err = l.s.Revisions(l.Namespace()).Get(context.TODO(), l)
		if err != nil {
			// reset the once so we can try again
			l.once = new(sync.Once)
			return
		}
		if cr, ok := rev.(*ConcreteRevision); ok {
			// can ignore err, concrete revisions will never error
			l.content, _ = cr.Content()
		} else {
			klog.Fatalf("lazy loading revision's store must return a concrete revision on get")
		}
	})
	return l.content, err
}

func ParseLazyLoadingRevision(s Storer, path string) (*LazyLoadingRevision, error) {
	split := strings.Split(path, "/")
	depth := len(split)
	if depth == 0 {
		return nil, fmt.Errorf("invalid path for revision: %s", path)
	}
	// special case cuz this has no group
	if split[0] == "v1" {
		split = append([]string{""}, split...)
		depth++
	}
	klog.V(4).Infof("parse lazy load: %v, depth: %d", split, depth)
	if depth == 5 {
		r, err := parseClusterScopedRevision(split)
		if err != nil {
			return nil, err
		}
		return &LazyLoadingRevision{
			revision: r,
			once:     new(sync.Once),
			s:        s,
		}, nil
	} else if depth == 6 {
		r, err := parseNamespacedRevision(split)
		if err != nil {
			return nil, err
		}
		return &LazyLoadingRevision{
			revision: r,
			once:     new(sync.Once),
			s:        s,
		}, nil
	}
	return nil, fmt.Errorf("invalid path for revision: %s", path)
}

func parseNamespacedRevision(split []string) (revision, error) {
	group, version, kind, namespace, name, file := split[0], split[1], split[2], split[3], split[4], split[5]
	fileSplit := strings.Split(file, "-")
	if len(fileSplit) < 2 {
		return revision{}, fmt.Errorf("invalid filename: %s, expected revision-timestamp.json", file)
	}
	resourceVersion, suffix := fileSplit[0], fileSplit[1]
	suffixSplit := strings.Split(suffix, ".")
	ts := suffixSplit[0]
	revisionTimestamp, err := strconv.Atoi(ts)
	if err != nil {
		return revision{}, err
	}

	rv, err := strconv.Atoi(resourceVersion)
	if err != nil {
		return revision{}, err
	}

	return revision{
		kind:              kind,
		apiVersion:        filepath.Join(group, version),
		name:              name,
		namespace:         namespace,
		resourceVersion:   rv,
		revisionTimestamp: time.Unix(int64(revisionTimestamp), 0),
		deleted:           strings.HasSuffix(suffix, "deleted"),
	}, nil
}

func parseClusterScopedRevision(split []string) (revision, error) {
	group, version, kind, name, file := split[0], split[1], split[2], split[3], split[4]
	fileSplit := strings.Split(file, "-")
	if len(fileSplit) < 2 {
		return revision{}, fmt.Errorf("invalid filename: %s, expected revision-timestamp.json", file)
	}
	resourceVersion, suffix := fileSplit[0], fileSplit[1]
	suffixSplit := strings.Split(suffix, ".")
	ts := suffixSplit[0]
	revisionTimestamp, err := strconv.Atoi(ts)
	if err != nil {
		return revision{}, err
	}

	rv, err := strconv.Atoi(resourceVersion)
	if err != nil {
		return revision{}, err
	}

	return revision{
		kind:              kind,
		apiVersion:        filepath.Join(group, version),
		name:              name,
		resourceVersion:   rv,
		revisionTimestamp: time.Unix(int64(revisionTimestamp), 0),
		deleted:           strings.HasSuffix(suffix, "deleted"),
	}, nil
}

func RevisionFrom(u *unstructured.Unstructured) *ConcreteRevision {
	rv, err := strconv.Atoi(u.GetResourceVersion())
	if err != nil {
		klog.Warningf("invalid resource resource version: %s, assuming 0 - bad things may happen.", u.GetResourceVersion())
	}
	return &ConcreteRevision{
		revision: revision{
			apiVersion:        u.GetAPIVersion(),
			kind:              u.GetKind(),
			namespace:         u.GetNamespace(),
			name:              u.GetName(),
			resourceVersion:   rv,
			revisionTimestamp: time.Now(),
			deleted:           false,
		},
		content: u,
	}
}

func (r revision) ResourceKey() string {
	return ResourceKey(r.apiVersion, r.kind, r.namespace, r.name)
}

func (r revision) RevisionKey() string {
	if !r.deleted {
		return fmt.Sprintf("%d-%d.json", r.resourceVersion, r.revisionTimestamp.Unix())
	}
	return fmt.Sprintf("%d-%d.json.deleted", r.resourceVersion, r.revisionTimestamp.Unix())
}

func (r revision) Key() string {
	return RevisionKey(r.ResourceKey(), r.RevisionKey())
}

type RevisionList []Revision

func (revs RevisionList) String() string {
	keys := make([]string, len(revs))
	for i := range revs {
		keys[i] = revs[i].String()
	}
	return fmt.Sprintf("[%s]", strings.Join(keys, ", "))
}

func (revs RevisionList) Cut(i, j int) RevisionList {
	copy(revs[i:], revs[j:])
	for k, n := len(revs)-j+i, len(revs); k < n; k++ {
		revs[k] = nil
	}
	return revs[:len(revs)-j+i]
}

func (revs RevisionList) Len() int      { return len(revs) }
func (revs RevisionList) Swap(x, y int) { revs[x], revs[y] = revs[y], revs[x] }
func (revs RevisionList) Less(x, y int) bool {
	return revs[x].ResourceVersion() < revs[y].ResourceVersion()
}

func (revs RevisionList) MarshalJSONWithContent(prefix, indent string) ([]byte, error) {
	out := bytes.Buffer{}
	i := 0
	out.WriteString(prefix)
	out.WriteRune('[')
	for _, rev := range revs {
		if i > 0 {
			out.WriteRune(',')
		}
		out.WriteRune('\n')
		out.WriteString(prefix)
		out.WriteString(indent)
		content, err := rev.Content()
		if err != nil {
			return nil, err
		}
		concreteRev := RevisionFrom(content)
		b, err := json.MarshalIndent(concreteRev, prefix+indent, indent)
		if err != nil {
			return nil, err
		}
		out.Write(b)
		i++
	}
	out.WriteRune('\n')
	out.WriteString(prefix)
	out.WriteRune(']')
	return out.Bytes(), nil
}

func RevisionKey(resourceKey string, revisionKey string) string {
	return filepath.Join(resourceKey, revisionKey)
}

func ResourceKey(apiVersion, kind, namespace, name string) string {
	return filepath.Join(apiVersion, kind, namespace, name)
}

func KeyPrefixForAPIVersionKindNamespace(apiVersion, kind, namespace string) string {
	return filepath.Join(apiVersion, kind, namespace)
}

func KeyPrefixForAPIVersionKind(apiVersion, kind string) string {
	return filepath.Join(apiVersion, kind)
}

func KeyPrefixForAPIVersion(apiVersion string) string {
	return apiVersion
}

func JoinKeyPrefix(prefixes ...string) string {
	return filepath.Join(prefixes...)
}

func StripMetadata(u *unstructured.Unstructured) {
	u.SetCreationTimestamp(metav1.Time{})
	u.SetDeletionTimestamp(nil)
	u.SetResourceVersion("")
	if err := unstructured.SetNestedField(u.Object, nil, "status"); err != nil {
		klog.Warningf("error stripping status field: %v", err)
	}
}
