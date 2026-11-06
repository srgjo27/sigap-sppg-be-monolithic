package infrastructure

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioObjectStore implements application.ObjectStore on any
// S3-compatible storage via minio-go/v7.
type MinioObjectStore struct {
	client *minio.Client
	bucket string
}

// NewMinioObjectStore builds the MinIO/S3 client from storage config.
func NewMinioObjectStore(endpoint, accessKey, secretKey, region string, useTLS bool, bucket string) (*MinioObjectStore, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useTLS,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("init object store: %w", err)
	}
	return &MinioObjectStore{client: client, bucket: bucket}, nil
}

// Put uploads the object with the known content length (no buffering).
func (s *MinioObjectStore) Put(ctx context.Context, key string, r io.Reader, size int64) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("put object %s: %w", key, err)
	}
	return nil
}

// Presign issues a presigned GET URL valid for ttl.
func (s *MinioObjectStore) Presign(ctx context.Context, key string, ttl time.Duration) (string, error) {
	url, err := s.client.PresignedGetObject(ctx, s.bucket, key, ttl, nil)
	if err != nil {
		return "", fmt.Errorf("presign object %s: %w", key, err)
	}
	return url.String(), nil
}

// Delete removes all listed objects in bulk.
func (s *MinioObjectStore) Delete(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	objectsCh := make(chan minio.ObjectInfo, len(keys))
	for _, key := range keys {
		objectsCh <- minio.ObjectInfo{Key: key}
	}
	close(objectsCh)
	for errItem := range s.client.RemoveObjects(ctx, s.bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if errItem.Err != nil {
			return fmt.Errorf("remove objects: %w", errItem.Err)
		}
	}
	return nil
}
