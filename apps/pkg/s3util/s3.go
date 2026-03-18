package s3util

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"gopkg.in/yaml.v3"
)

type S3Config struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"accessKey"`
	SecretKey string `yaml:"secretKey"`
	UseSSL    bool   `yaml:"useSSL"`
	Region    string `yaml:"region"`
	Bucket    string `yaml:"bucket"`
}

var (
	clients       = map[string]*minio.Client{}
	defaultClient *minio.Client
	defaultBucket string
	mu            sync.RWMutex
)

func Init(ctx context.Context, config map[string]S3Config) error {
	if len(config) == 0 {
		return fmt.Errorf("s3 config is empty")
	}

	var firstClient *minio.Client
	var firstBucket string
	for name, cfg := range config {
		client, err := initClient(ctx, name, cfg)
		if err != nil {
			return err
		}
		if firstClient == nil {
			firstClient = client
			firstBucket = cfg.Bucket
		}
	}

	defaultClient = firstClient
	defaultBucket = firstBucket
	return nil
}

func ParseS3Config(config map[string]string) (map[string]S3Config, error) {
	s3 := make(map[string]S3Config, len(config))
	var s3ConfigValue S3Config
	for k, v := range config {
		err := yaml.Unmarshal([]byte(v), &s3ConfigValue)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal build config %v: %v", k, err)
		}
		s3[k] = s3ConfigValue
	}
	return s3, nil
}

func GetClient(name string) (*minio.Client, error) {
	mu.RLock()
	defer mu.RUnlock()

	client, ok := clients[name]
	if !ok {
		return nil, fmt.Errorf("s3 client %s not initialized", name)
	}
	return client, nil
}

func EnsureBucket(ctx context.Context, bucketName string) error {
	client, err := getDefaultClient()
	if err != nil {
		return err
	}

	exists, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("could not check if bucket %s exists: %v", bucketName, err)
	}
	if exists {
		return nil
	}

	if err := client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("could not create bucket %s: %v", bucketName, err)
	}
	return nil
}

func PutObject(ctx context.Context, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	client, bucketName, err := getDefaultTarget()
	if err != nil {
		return minio.UploadInfo{}, err
	}
	return client.PutObject(ctx, bucketName, objectName, reader, objectSize, opts)
}

func GetObject(ctx context.Context, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	client, bucketName, err := getDefaultTarget()
	if err != nil {
		return nil, err
	}
	object, err := client.GetObject(ctx, bucketName, objectName, opts)
	if err != nil {
		return nil, err
	}
	return object, nil
}

func RemoveObject(ctx context.Context, objectName string, opts minio.RemoveObjectOptions) error {
	client, bucketName, err := getDefaultTarget()
	if err != nil {
		return err
	}
	return client.RemoveObject(ctx, bucketName, objectName, opts)
}

func StatObject(ctx context.Context, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	client, bucketName, err := getDefaultTarget()
	if err != nil {
		return minio.ObjectInfo{}, err
	}
	return client.StatObject(ctx, bucketName, objectName, opts)
}

func ListObjects(ctx context.Context, opts minio.ListObjectsOptions) (<-chan minio.ObjectInfo, error) {
	client, bucketName, err := getDefaultTarget()
	if err != nil {
		return nil, err
	}
	return client.ListObjects(ctx, bucketName, opts), nil
}

func initClient(ctx context.Context, name string, config S3Config) (*minio.Client, error) {
	if strings.TrimSpace(config.Endpoint) == "" {
		return nil, fmt.Errorf("s3 config %s missing endpoint", name)
	}
	if strings.TrimSpace(config.AccessKey) == "" {
		return nil, fmt.Errorf("s3 config %s missing accessKey", name)
	}
	if strings.TrimSpace(config.SecretKey) == "" {
		return nil, fmt.Errorf("s3 config %s missing secretKey", name)
	}

	slog.Info("initializing s3", "name", name, "endpoint", config.Endpoint, "bucket", config.Bucket, "useSSL", config.UseSSL)
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseSSL,
		Region: config.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create s3 client %s: %v", name, err)
	}

	mu.Lock()
	clients[name] = client
	mu.Unlock()

	if config.Bucket != "" {
		exists, err := client.BucketExists(ctx, config.Bucket)
		if err != nil {
			return nil, fmt.Errorf("could not check s3 bucket %s: %v", config.Bucket, err)
		}
		if !exists {
			slog.Info("configured s3 bucket does not exist yet", "name", name, "bucket", config.Bucket)
		}
	}

	return client, nil
}

func getDefaultClient() (*minio.Client, error) {
	if defaultClient == nil {
		return nil, fmt.Errorf("s3 client not initialized")
	}
	return defaultClient, nil
}

func getDefaultTarget() (*minio.Client, string, error) {
	client, err := getDefaultClient()
	if err != nil {
		return nil, "", err
	}
	if strings.TrimSpace(defaultBucket) == "" {
		return nil, "", fmt.Errorf("default s3 bucket not configured")
	}
	return client, defaultBucket, nil
}
