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

package config

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/trinchan/saiki/pkg/util/sliceutil"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	DefaultConfig = "config.json"
	// If we change the config or store format to something incompatible,
	// we can bump this and write shims to decode both. Hopefully.
	DefaultVersion = 1
)

type Configger interface {
	WriteConfig(context.Context, *Config) error
	ReadConfig(context.Context) (*Config, error)
}

type Config struct {
	Resources []*metav1.APIResourceList `json:"resources"`
	Version   int                       `json:"version"`
	// TODO Should we add this? Thought it would be nice to have in clear text
	// what algorithm was used to encrypt in case you have a store and you forgot your
	// settings. Would require that the store/backend expose the encryption algorithm to the
	// controller.. or some other refactor.
	// EncryptionAlgorithm string                    `json:"encryptionAlgorithm"`
	// Hasher              crypto.Hash               `json:"hasher"`
}

func (c *Config) BestResourceMappingFor(resource string) (string, metav1.APIResource, error) {
	match := strings.ToLower(resource)
	var potentialGroupVersion string
	var potentialAPIResource metav1.APIResource
	for _, groupVersion := range c.Resources {
		gv, err := schema.ParseGroupVersion(groupVersion.GroupVersion)
		if err != nil {
			continue
		}
		for _, a := range groupVersion.APIResources {
			if match == a.SingularName+"."+gv.Group {
				return groupVersion.GroupVersion, a, nil
			}
			if strings.ToLower(a.Kind) == match || strings.ToLower(a.SingularName) == match || strings.ToLower(a.Name) == match || sliceutil.In(a.ShortNames, match) {
				potentialGroupVersion, potentialAPIResource = groupVersion.GroupVersion, a
			}
		}
	}
	if potentialAPIResource.Kind == "" {
		return "", potentialAPIResource, fmt.Errorf("the store doesn't have a resource type %q", resource)
	}
	return potentialGroupVersion, potentialAPIResource, nil
}
