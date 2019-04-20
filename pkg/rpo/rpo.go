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

package rpo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	saikierr "github.com/trinchan/saiki/pkg/errors"
	"github.com/trinchan/saiki/pkg/store"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/klog"
)

type Duration time.Duration

func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}
}

// TODO maybe add some helper methods to this struct so users of it
// don't have to implement their own methods that determine whether
// a policy applies to them.
// e.g. RPOPolicy.ShouldPurge(Revision)
type RPOPolicy struct {
	PurgeOlderThan Duration `json:"purgeOlderThan"`
	RPO            Duration `json:"rpo"`
}

func Rotate(ctx context.Context, s store.RevisionGetter, rev store.Revision, policy RPOPolicy) (store.RevisionList, error) {
	toPurge := make(map[store.Revision]struct{})
	klog.V(3).Infof("rotating: %s - %+v", rev, policy)
	revisions, err := s.Revisions(rev.Namespace()).List(ctx, rev.TypeMeta(), rev.Name(), metav1.ListOptions{})
	if err == saikierr.ErrNotFound {
		klog.V(3).Infof("rotating: %s - list returned no revisions", rev)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	klog.V(3).Infof("revisions: %s - %d", rev, len(revisions))
	for _, rev := range revisions {
		klog.V(3).Infof("revision timestamp: %s", duration.HumanDuration(time.Since(rev.RevisionTimestamp())))
	}
	if len(revisions) == 0 {
		return nil, nil
	}
	// TODO some active rotation logic can be put here, nothing yet
	if len(toPurge) == 0 {
		return nil, nil
	}
	ret := make(store.RevisionList, 0, len(toPurge))
	i := 0
	for revision := range toPurge {
		ret[i] = revision
		i++
	}
	return ret, nil
}
