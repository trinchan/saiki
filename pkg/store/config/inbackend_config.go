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
	"encoding/json"

	"github.com/trinchan/saiki/pkg/store/backend"
	"k8s.io/klog"
)

type inBackend struct {
	backend backend.Backend
}

func NewInBackend(b backend.Backend) *inBackend {
	return &inBackend{
		backend: b,
	}
}

func (s *inBackend) ReadConfig(ctx context.Context) (*Config, error) {
	f, err := s.backend.Open(ctx, DefaultConfig)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	config := new(Config)
	err = json.NewDecoder(f).Decode(config)
	return config, err
}

func (s *inBackend) WriteConfig(ctx context.Context, config *Config) error {
	if config.Version == 0 {
		config.Version = DefaultVersion
	}
	klog.V(4).Infof("writing config")
	f, err := s.backend.Create(ctx, DefaultConfig)
	if err != nil {
		return err
	}
	defer f.Close()
	err = json.NewEncoder(f).Encode(config)
	klog.V(4).Infof("wrote config")
	return err
}
