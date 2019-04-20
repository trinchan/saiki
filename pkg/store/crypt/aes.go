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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"io/ioutil"

	"github.com/pkg/errors"
	"k8s.io/klog"
)

func NewAESEncrypter(key string) Encrypter {
	if len(key) == 0 {
		klog.Fatal("aes encryption key cannot be blank")
	}
	encrypter := &AESEncrypter{}
	if crypto.SHA256.Available() {
		encrypter.HashAlgorithm = crypto.SHA256
	}

	if encrypter.HashAlgorithm == crypto.Hash(0) {
		klog.Fatal("could not find compatible hash algorithm")
	}

	hash, err := createHash(encrypter.HashAlgorithm, key)
	if err != nil {
		klog.Fatalf(errors.Wrap(err, "creating crypto hash").Error())
	}
	block, err := aes.NewCipher(hash)
	if err != nil {
		klog.Fatalf(errors.Wrap(err, "creating aes cipher").Error())
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		klog.Fatalf(errors.Wrap(err, "creating gcm").Error())
	}

	encrypter.AEAD = gcm
	return encrypter
}

type AESEncrypter struct {
	HashAlgorithm crypto.Hash
	AEAD          cipher.AEAD
}

func (a *AESEncrypter) Encrypt(r io.Reader) ([]byte, error) {
	nonce := make([]byte, a.AEAD.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	ciphertext := a.AEAD.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

func (a *AESEncrypter) Decrypt(r io.Reader) ([]byte, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	nonceSize := a.AEAD.NonceSize()
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return a.AEAD.Open(nil, nonce, ciphertext, nil)
}

func (a *AESEncrypter) Algorithm() string {
	return "AES"
}

func (a *AESEncrypter) Hasher() crypto.Hash {
	return a.HashAlgorithm
}
