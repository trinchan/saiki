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

package policy

import (
	"strconv"

	"github.com/trinchan/saiki/pkg/rpo"
	"github.com/trinchan/saiki/pkg/store"
	"github.com/trinchan/saiki/pkg/util/sliceutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog"
)

type BackupPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Rules holds all the PolicyRules for this BackupPolicy
	// +optional
	Rules []BackupRule `json:"rules"`

	Fields []string `json:"fields"`

	RPOPolicy *rpo.RPOPolicy `json:"rpoPolicy,omitempty"`
}

type BackupRule struct {
	APIGroups []string `json:"apiGroups,omitempty"`
	// Resources is a list of resources this rule applies to.  ResourceAll represents all resources.
	// +optional
	Resources []string `json:"resources,omitempty"`
	// ResourceNames is an optional white list of names that the rule applies to.  An empty set means that everything is allowed.
	// +optional
	ResourceNames []string `json:"resourceNames,omitempty"`

	Namespaces []string `json:"namespaces,omitempty"`

	Fields []string `json:"fields"`

	LabelSelector *PolicyLabelSelector `json:"labelSelector,omitempty"`

	RPOPolicy *rpo.RPOPolicy `json:"rpoPolicy,omitempty"`
}

type PolicyLabelSelector struct {
	labels.Selector
}

func (p *PolicyLabelSelector) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	unquoted, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	selector, err := labels.Parse(unquoted)
	if err != nil {
		return err
	}
	p.Selector = selector
	return nil
}

func (b BackupPolicy) RuleFor(rev store.Revision) *BackupRule {
	specificity := 0
	var ret *BackupRule
	var nameMatch, namespaceMatch, resourcesMatch, groupMatch, labelMatch bool
	for _, rule := range b.Rules {
		r := rule
		// TODO if we sort by specificity on load, we can easily ook for matches in descending specificity order
		// and return the first one that matches instead of having to check all of them
		nameMatch = sliceutil.In(rule.ResourceNames, rev.Name()) || len(rule.ResourceNames) == 0 || (len(rule.ResourceNames) > 0 && rule.ResourceNames[0] == "*")
		namespaceMatch = sliceutil.In(rule.Namespaces, rev.Namespace()) || len(rule.Namespaces) == 0 || (len(rule.Namespaces) > 0 && rule.Namespaces[0] == "*")
		resourcesMatch = sliceutil.In(rule.Resources, rev.Kind()) || len(rule.Resources) == 0 || (len(rule.Resources) > 0 && rule.Resources[0] == "*")
		groupMatch = sliceutil.In(rule.APIGroups, rev.APIVersion()) || len(rule.APIGroups) == 0 || (len(rule.APIGroups) > 0 && rule.APIGroups[0] == "*")
		if rule.LabelSelector == nil {
			labelMatch = true
		} else {
			content, err := rev.Content()
			if err != nil {
				klog.Warningf("error getting content for rev: %s - %v", rev, err)
				continue
			}
			labelMatch = rule.LabelSelector.Matches(labels.Set(content.GetLabels()))
		}

		if nameMatch && namespaceMatch && resourcesMatch && groupMatch && labelMatch {
			ret = &r
			break
		}

		if specificity < 4 && namespaceMatch && resourcesMatch && groupMatch && labelMatch {
			ret = &r
			specificity = 3
		}

		if specificity < 3 && resourcesMatch && groupMatch && labelMatch {
			ret = &r
			specificity = 2
		}

		if specificity < 2 && groupMatch && labelMatch {
			ret = &r
			specificity = 1
		}
	}
	if ret == nil {
		return nil
	}
	ret.Fields = append(ret.Fields, b.Fields...)
	if ret.RPOPolicy == nil {
		ret.RPOPolicy = b.RPOPolicy
	}
	if ret.RPOPolicy.RPO == 0 {
		ret.RPOPolicy.RPO = b.RPOPolicy.RPO
	}
	if ret.RPOPolicy.PurgeOlderThan == 0 {
		ret.RPOPolicy.PurgeOlderThan = b.RPOPolicy.PurgeOlderThan
	}
	klog.V(5).Infof("final rule: %+v", ret)
	klog.V(5).Infof("final rpo policy: %+v", ret.RPOPolicy)
	return ret
}
