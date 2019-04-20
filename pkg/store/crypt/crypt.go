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

package crypt

import (
	"crypto"
	"fmt"
	"io"

	"github.com/spf13/viper"
)

var (
	EncrypterMap = map[string]EncrypterFunc{
		"AES": NewAESEncrypter,
	}
	Providers = []string{
		"AES",
	}
)

type Encrypter interface {
	Encrypt(io.Reader) ([]byte, error)
	Decrypt(io.Reader) ([]byte, error)
	Algorithm() string
	Hasher() crypto.Hash
}

func createHash(hash crypto.Hash, key string) ([]byte, error) {
	hasher := hash.New()
	_, err := hasher.Write([]byte(key))
	if err != nil {
		return nil, err
	}
	return hasher.Sum(nil), nil
}

type EncrypterFunc func(key string) Encrypter

func New(algorithm, key string) (Encrypter, error) {
	if encrypter, ok := EncrypterMap[algorithm]; ok {
		return encrypter(key), nil
	}
	return nil, fmt.Errorf("encrypter not found with algorithm: %s", algorithm)
}

func NewFromEnv() (Encrypter, error) {
	algorithm, key := viper.GetString("encryption-algorithm"), viper.GetString("encryption-key")
	if algorithm == "" || key == "" {
		return nil, nil
	}
	return New(algorithm, key)
}
