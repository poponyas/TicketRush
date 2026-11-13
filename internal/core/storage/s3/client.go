// Package s3 provides the S3 adapter used for private poster objects.
package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	coreerrors "github.com/poponyas/TicketRush/internal/core/errors"
)

type Config struct {
	Endpoint  string        `envconfig:"S3_ENDPOINT" default:"127.0.0.1:9002"`
	AccessKey string        `envconfig:"S3_ACCESS_KEY" default:"ticketrush"`
	SecretKey string        `envconfig:"S3_SECRET_KEY" default:"ticketrush_dev"`
	Bucket    string        `envconfig:"S3_BUCKET" default:"ticketrush-posters"`
	Secure    bool          `envconfig:"S3_SECURE" default:"false"`
	Timeout   time.Duration `envconfig:"S3_TIMEOUT" default:"5s"`
}

func NewConfig() (Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return cfg, fmt.Errorf("S3 config: %w", err)
	}
	return cfg, nil
}

type Store struct {
	client  *minio.Client
	bucket  string
	timeout time.Duration
}

func New(cfg Config) (*Store, error) {
	if cfg.Endpoint == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Bucket == "" || cfg.Timeout <= 0 {
		return nil, fmt.Errorf("invalid S3 configuration")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: cfg.Secure})
	if err != nil {
		return nil, fmt.Errorf("S3 client: %w", err)
	}
	return &Store{client: client, bucket: cfg.Bucket, timeout: cfg.Timeout}, nil
}

// EnsureBucket is called by setup, not by the API: storage outages must not stop
// unrelated booking/authentication requests from being served.
func (s *Store) EnsureBucket(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("check bucket: %w", err)
	}
	if exists {
		return nil
	}
	err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
	if code := minio.ToErrorResponse(err).Code; code == "BucketAlreadyOwnedByYou" {
		return nil
	}
	return err
}
func (s *Store) Put(ctx context.Context, key string, data []byte, contentType string) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("put poster: %w: %w", coreerrors.ErrUnavailable, err)
	}
	return nil
}
func (s *Store) Get(ctx context.Context, key string) ([]byte, string, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, "", storageError(err)
	}
	defer object.Close()
	info, err := object.Stat()
	if err != nil {
		return nil, "", storageError(err)
	}
	const maxSize = 5 << 20
	if info.Size > maxSize {
		return nil, "", fmt.Errorf("stored poster is too large")
	}
	data, err := io.ReadAll(io.LimitReader(object, maxSize+1))
	if err != nil {
		return nil, "", storageError(err)
	}
	if len(data) > maxSize {
		return nil, "", fmt.Errorf("stored poster is too large")
	}
	return data, info.ContentType, nil
}
func (s *Store) Delete(ctx context.Context, key string) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}
func storageError(err error) error {
	if minio.ToErrorResponse(err).Code == "NoSuchKey" {
		return coreerrors.ErrNotFound
	}
	return fmt.Errorf("get poster: %w: %w", coreerrors.ErrUnavailable, err)
}
