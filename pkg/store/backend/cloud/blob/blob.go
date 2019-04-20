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

package blob

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/spf13/viper"
	"gocloud.dev/blob"
	"gocloud.dev/blob/fileblob"
	"gocloud.dev/blob/memblob"
	"gocloud.dev/blob/s3blob"
	"k8s.io/klog"
)

type Provider string

const (
	S3Provider  Provider = "S3"
	MemProvider Provider = "memory"
	FSProvider  Provider = "filesystem"
)

var (
	Providers = []string{
		string(S3Provider),
		string(MemProvider),
		string(FSProvider),
	}
)

var (
	ProviderMap = map[string]Provider{
		string(S3Provider):  S3Provider,
		string(MemProvider): MemProvider,
		string(FSProvider):  FSProvider,
	}
)

type Backend struct {
	provider Provider
	config   Config
	bucket   *blob.Bucket
}

type OptionFunc func(*Backend)

func WithProvider(provider Provider) OptionFunc {
	return func(b *Backend) {
		b.provider = provider
	}
}

func WithConfig(config Config) OptionFunc {
	return func(b *Backend) {
		b.config = config
	}
}

func newS3Provider(ctx context.Context, config Config) (*blob.Bucket, error) {
	if config.S3 == nil {
		return nil, fmt.Errorf("s3 config empty")
	}
	sess, err := session.NewSession(config.S3)
	if err != nil {
		return nil, err
	}
	return s3blob.OpenBucket(ctx, sess, config.BucketName, nil)
}

func newMemProvider(config Config) *blob.Bucket {
	return memblob.OpenBucket(config.Mem)
}

func newFSProvider(config Config) (*blob.Bucket, error) {
	return fileblob.OpenBucket(config.Root, config.FS)
}

func New(ctx context.Context, opts ...OptionFunc) (*Backend, error) {
	b := &Backend{}
	for _, o := range opts {
		o(b)
	}
	var err error
	switch b.provider {
	case S3Provider:
		b.bucket, err = newS3Provider(ctx, b.config)
	case MemProvider:
		b.bucket = newMemProvider(b.config)
	case FSProvider:
		b.bucket, err = newFSProvider(b.config)
	default:
		err = fmt.Errorf("implementation not found for provider: %q", b.provider)
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

func NewFromEnv(ctx context.Context) (*Backend, error) {
	provider := viper.GetString("blob-provider")
	p, ok := ProviderMap[provider]
	if !ok {
		return nil, fmt.Errorf("provider implementation not found for provider: %s", provider)
	}

	return New(ctx, WithProvider(p), WithConfig(ConfigFromEnv(p)))
}

func (b *Backend) Create(ctx context.Context, name string) (io.WriteCloser, error) {
	klog.V(5).Infof("blob backend writer: %s", name)
	return b.bucket.NewWriter(ctx, name, nil)
}

func (b *Backend) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	klog.V(2).Infof("blob backend read: %s", name)
	return b.bucket.NewReader(ctx, name, nil)
}

func (b *Backend) ReadDir(ctx context.Context, path string) ([]os.FileInfo, error) {
	var ret []os.FileInfo
	klog.V(2).Infof("blob backend read dir: %s", path)
	i := b.bucket.List(&blob.ListOptions{
		Prefix:    path + "/",
		Delimiter: "/",
	})
	for {
		obj, err := i.Next(ctx)
		klog.V(5).Infof("obj: %+v", obj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if !obj.IsDir {
			ret = append(ret, &FileInfo{obj})
		}
	}
	return ret, nil
}

func (b *Backend) Remove(ctx context.Context, key string) error {
	klog.V(5).Infof("blob backend delete: %s", key)
	return b.bucket.Delete(ctx, key)
}

func (b *Backend) RemoveAll(ctx context.Context, keys []string) ([]string, error) {
	klog.V(5).Infof("blob backend bulk delete: %v", keys)
	var deletedKeys []string
	switch b.provider {
	case S3Provider:
		var s3Bucket *s3.S3
		var err error
		if b.bucket.As(&s3Bucket) {
			chunkSize := 1000
			for i := 0; i < len(keys)/chunkSize+1; i++ {
				startChunk := i * chunkSize
				endChunk := i*chunkSize + chunkSize
				if endChunk > len(keys) {
					endChunk = len(keys)
				}
				objects := make([]*s3.ObjectIdentifier, endChunk-startChunk)
				chunk := keys[startChunk:endChunk]
				for i, key := range chunk {
					objects[i] = &s3.ObjectIdentifier{
						Key: aws.String(key),
					}
				}
				deleteObjectsInput := &s3.DeleteObjectsInput{
					Bucket: aws.String(b.config.BucketName),
					Delete: &s3.Delete{
						Objects: objects,
					},
				}
				out, err := s3Bucket.DeleteObjects(deleteObjectsInput)
				if out == nil {
					return nil, err
				}
				if err != nil {
					klog.Warningf("erroring during S3 RemoveAll: %v", err)
				}
				for _, deleted := range out.Deleted {
					if deleted.Key != nil {
						deletedKeys = append(deletedKeys, *deleted.Key)
					}
				}
			}
			return deletedKeys, err
		}
		return nil, fmt.Errorf("error converting blob.Bucket to s3.Bucket")
	// TODO figure out how to organize the code so that this shim is not needed
	// should be able to expose whether the Backend implementation is a BulkRemovingBackend
	// we could copy the backend.Backend and BulkRemovingBackend interfaces into this package
	// but that's not a good solution.
	default:
		var err error
		for _, key := range keys {
			err = b.Remove(ctx, key)
			if err != nil {
				klog.Warningf("error deleting from blob store: %v", err)
			} else {
				deletedKeys = append(deletedKeys, key)
			}
		}
		return deletedKeys, err
	}
}

func (b *Backend) Provider() string {
	return string(b.provider)
}

func (b *Backend) Root() string {
	return b.config.Root
}

func (b *Backend) Close(ctx context.Context) error {
	klog.V(2).Infof("closing blob storage")
	return b.bucket.Close()
}

func (b *Backend) Walk(ctx context.Context, root string, walkFn filepath.WalkFunc) error {
	klog.V(3).Infof("blob backend walk: %q", root)
	skippedDirs := make(map[string]bool)
	i := b.bucket.List(&blob.ListOptions{
		Prefix:    root,
		Delimiter: "",
	})

	var walkErr error
	for {
		obj, nextErr := i.Next(ctx)
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nextErr
		}
		dir := filepath.Dir(obj.Key)
		if skippedDirs[dir] {
			klog.V(5).Infof("skipped during walk: %s", obj.Key)
			continue
		}
		walkErr = walkFn(obj.Key, &FileInfo{obj}, nil)
		if walkErr != nil && walkErr != filepath.SkipDir {
			return walkErr
		}
		if walkErr == filepath.SkipDir {
			klog.V(5).Infof("will skip dirs during walk: %s", obj.Key)
			skippedDirs[dir] = true
		}
	}
	return nil
}

type FileInfo struct {
	*blob.ListObject
}

func (f *FileInfo) IsDir() bool {
	return f.ListObject.IsDir
}

func (f *FileInfo) ModTime() time.Time {
	return f.ListObject.ModTime
}

func (f *FileInfo) Name() string {
	return filepath.Base(f.ListObject.Key)
}

func (f *FileInfo) Size() int64 {
	return f.ListObject.Size
}

func (f *FileInfo) Mode() os.FileMode {
	if f.IsDir() {
		return os.ModeDir
	}
	return os.FileMode(0)
}

func (f *FileInfo) Sys() interface{} {
	return nil
}
