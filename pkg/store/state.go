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
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StateOptions struct {
	metav1.TypeMeta
	LabelSelector *metav1.LabelSelector
	Time          time.Time
	Namespace     string
	AllNamespaces bool
	Name          string
	AllRevisions  bool
	OnlyDeleted   bool
}

type State map[string]RevisionList

func (s State) MarshalJSONOnlyContent(prefix, indent string) ([]byte, error) {
	var out bytes.Buffer
	i := 0
	for _, revisionlist := range s {
		for _, rev := range revisionlist {
			if i > 0 {
				if _, err := out.WriteRune('\n'); err != nil {
					return nil, err
				}
			}
			content, err := rev.Content()
			if err != nil {
				return nil, err
			}
			b, err := json.MarshalIndent(content, prefix, indent)
			if err != nil {
				return nil, err
			}
			_, err = out.Write(b)
			if err != nil {
				return nil, err
			}
			i++
		}
	}
	return out.Bytes(), nil
}

func (s State) MarshalJSONWithContent(prefix, indent string) ([]byte, error) {
	out := bytes.Buffer{}
	i := 0
	out.WriteString(prefix)
	out.WriteRune('{')
	for key, revisionlist := range s {
		if i > 0 {
			out.WriteRune(',')
		}
		out.WriteRune('\n')
		out.WriteString(prefix)
		out.WriteString(indent)
		out.WriteString(fmt.Sprintf("%q: ", key))
		b, err := revisionlist.MarshalJSONWithContent(prefix+indent, indent)
		if err != nil {
			return nil, err
		}
		out.Write(b)
		i++
	}
	out.WriteRune('\n')
	out.WriteString(prefix)
	out.WriteRune('}')
	return out.Bytes(), nil
}
