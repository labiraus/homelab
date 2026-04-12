package minioutil

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"accessKey"`
	SecretKey string `yaml:"secretKey"`
	UseSSL    bool   `yaml:"useSSL"`
	Region    string `yaml:"region"`
	Bucket    string `yaml:"bucket"`
}

type Object struct {
	Info minio.ObjectInfo
	Body []byte
}

type FolderEntry struct {
	Name         string    `json:"name"`
	Type         string    `json:"type"`
	ObjectKey    string    `json:"objectKey,omitempty"`
	Prefix       string    `json:"prefix,omitempty"`
	SizeBytes    int64     `json:"sizeBytes,omitempty"`
	LastModified time.Time `json:"lastModified,omitempty"`
	ContentType  string    `json:"contentType,omitempty"`
}

var (
	clients       = map[string]*minio.Client{}
	defaultClient *minio.Client
	defaultBucket string
	mu            sync.RWMutex
)

func Init(ctx context.Context, config map[string]Config) error {
	if len(config) == 0 {
		return fmt.Errorf("minio config is empty")
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

func ParseConfig(config map[string]string) (map[string]Config, error) {
	parsed := make(map[string]Config, len(config))
	var configValue Config
	for key, value := range config {
		if err := yaml.Unmarshal([]byte(value), &configValue); err != nil {
			return nil, fmt.Errorf("could not unmarshal minio config %v: %v", key, err)
		}
		parsed[key] = configValue
	}
	return parsed, nil
}

func GetClient(name string) (*minio.Client, error) {
	mu.RLock()
	defer mu.RUnlock()

	client, ok := clients[name]
	if !ok {
		return nil, fmt.Errorf("minio client %s not initialized", name)
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
	client, bucketName, err := getTarget("")
	if err != nil {
		return minio.UploadInfo{}, err
	}
	return client.PutObject(ctx, bucketName, objectName, reader, objectSize, opts)
}

func PutObjectToBucket(ctx context.Context, bucketName string, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	client, resolvedBucket, err := getTarget(bucketName)
	if err != nil {
		return minio.UploadInfo{}, err
	}
	return client.PutObject(ctx, resolvedBucket, objectName, reader, objectSize, opts)
}

func GetObject(ctx context.Context, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	client, bucketName, err := getTarget("")
	if err != nil {
		return nil, err
	}
	object, err := client.GetObject(ctx, bucketName, objectName, opts)
	if err != nil {
		return nil, err
	}
	return object, nil
}

func GetObjectFromBucket(ctx context.Context, bucketName string, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	client, resolvedBucket, err := getTarget(bucketName)
	if err != nil {
		return nil, err
	}
	object, err := client.GetObject(ctx, resolvedBucket, objectName, opts)
	if err != nil {
		return nil, err
	}
	return object, nil
}

func RemoveObject(ctx context.Context, objectName string, opts minio.RemoveObjectOptions) error {
	client, bucketName, err := getTarget("")
	if err != nil {
		return err
	}
	return client.RemoveObject(ctx, bucketName, objectName, opts)
}

func RemoveObjectFromBucket(ctx context.Context, bucketName string, objectName string, opts minio.RemoveObjectOptions) error {
	client, resolvedBucket, err := getTarget(bucketName)
	if err != nil {
		return err
	}
	return client.RemoveObject(ctx, resolvedBucket, objectName, opts)
}

func StatObject(ctx context.Context, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	client, bucketName, err := getTarget("")
	if err != nil {
		return minio.ObjectInfo{}, err
	}
	return client.StatObject(ctx, bucketName, objectName, opts)
}

func StatObjectInBucket(ctx context.Context, bucketName string, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	client, resolvedBucket, err := getTarget(bucketName)
	if err != nil {
		return minio.ObjectInfo{}, err
	}
	return client.StatObject(ctx, resolvedBucket, objectName, opts)
}

func ListObjects(ctx context.Context, opts minio.ListObjectsOptions) (<-chan minio.ObjectInfo, error) {
	client, bucketName, err := getTarget("")
	if err != nil {
		return nil, err
	}
	return client.ListObjects(ctx, bucketName, opts), nil
}

func ListObjectsInBucket(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) (<-chan minio.ObjectInfo, error) {
	client, resolvedBucket, err := getTarget(bucketName)
	if err != nil {
		return nil, err
	}
	return client.ListObjects(ctx, resolvedBucket, opts), nil
}

func ListObjectInfoInBucket(ctx context.Context, bucketName string, opts minio.ListObjectsOptions, maxKeys int) ([]minio.ObjectInfo, error) {
	listCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	objects, err := ListObjectsInBucket(listCtx, bucketName, opts)
	if err != nil {
		return nil, err
	}

	objectInfo := []minio.ObjectInfo{}
	for object := range objects {
		if object.Err != nil {
			return nil, object.Err
		}

		objectInfo = append(objectInfo, object)
		if maxKeys > 0 && len(objectInfo) >= maxKeys {
			cancel()
			break
		}
	}

	return objectInfo, nil
}

func ReadObjectFromBucket(ctx context.Context, bucketName string, objectName string) (Object, error) {
	info, err := StatObjectInBucket(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return Object{}, err
	}

	object, err := GetObjectFromBucket(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return Object{}, err
	}
	defer object.Close()

	body, err := io.ReadAll(object)
	if err != nil {
		return Object{}, err
	}

	return Object{
		Info: info,
		Body: body,
	}, nil
}

func PutTextObjectToBucket(ctx context.Context, bucketName string, objectName string, text string, opts minio.PutObjectOptions) (minio.ObjectInfo, error) {
	if strings.TrimSpace(opts.ContentType) == "" {
		opts.ContentType = "text/plain; charset=utf-8"
	}

	if _, err := PutObjectToBucket(
		ctx,
		bucketName,
		objectName,
		strings.NewReader(text),
		int64(len(text)),
		opts,
	); err != nil {
		return minio.ObjectInfo{}, err
	}

	return StatObjectInBucket(ctx, bucketName, objectName, minio.StatObjectOptions{})
}

func PutObjectBytesToBucket(ctx context.Context, bucketName string, objectName string, body []byte, opts minio.PutObjectOptions) (minio.ObjectInfo, error) {
	if _, err := PutObjectToBucket(
		ctx,
		bucketName,
		objectName,
		bytes.NewReader(body),
		int64(len(body)),
		opts,
	); err != nil {
		return minio.ObjectInfo{}, err
	}

	return StatObjectInBucket(ctx, bucketName, objectName, minio.StatObjectOptions{})
}

func DeleteObjectFromBucket(ctx context.Context, bucketName string, objectName string) error {
	return RemoveObjectFromBucket(ctx, bucketName, objectName, minio.RemoveObjectOptions{})
}

func ListFolderEntriesInBucket(ctx context.Context, bucketName string, prefix string, maxKeys int) ([]FolderEntry, error) {
	normalizedPrefix := normalizePrefix(prefix)
	objects, err := ListObjectInfoInBucket(ctx, bucketName, minio.ListObjectsOptions{
		Prefix:    normalizedPrefix,
		Recursive: true,
	}, 0)
	if err != nil {
		return nil, err
	}

	folders := map[string]FolderEntry{}
	files := []FolderEntry{}

	for _, object := range objects {
		objectKey := strings.TrimSpace(object.Key)
		if objectKey == "" || strings.HasSuffix(objectKey, "/") {
			continue
		}

		relativeKey := strings.TrimPrefix(objectKey, normalizedPrefix)
		if relativeKey == "" {
			continue
		}

		parts := strings.Split(relativeKey, "/")
		if len(parts) > 1 {
			folderPrefix := normalizedPrefix + parts[0] + "/"
			if _, exists := folders[folderPrefix]; exists {
				continue
			}
			folders[folderPrefix] = FolderEntry{
				Name:   parts[0],
				Type:   "folder",
				Prefix: folderPrefix,
			}
			continue
		}

		files = append(files, FolderEntry{
			Name:         parts[0],
			Type:         "file",
			ObjectKey:    objectKey,
			SizeBytes:    object.Size,
			LastModified: object.LastModified.UTC(),
			ContentType:  object.ContentType,
		})
	}

	entries := make([]FolderEntry, 0, len(folders)+len(files))
	for _, folder := range folders {
		entries = append(entries, folder)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})
	entries = append(entries, files...)

	if maxKeys > 0 && len(entries) > maxKeys {
		entries = entries[:maxKeys]
	}

	return entries, nil
}

func initClient(ctx context.Context, name string, config Config) (*minio.Client, error) {
	if strings.TrimSpace(config.Endpoint) == "" {
		return nil, fmt.Errorf("minio config %s missing endpoint", name)
	}
	if strings.TrimSpace(config.AccessKey) == "" {
		return nil, fmt.Errorf("minio config %s missing accessKey", name)
	}
	if strings.TrimSpace(config.SecretKey) == "" {
		return nil, fmt.Errorf("minio config %s missing secretKey", name)
	}

	slog.Info("initializing minio", "name", name, "endpoint", config.Endpoint, "bucket", config.Bucket, "useSSL", config.UseSSL)
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseSSL,
		Region: config.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create minio client %s: %v", name, err)
	}

	mu.Lock()
	clients[name] = client
	mu.Unlock()

	if config.Bucket != "" {
		exists, err := client.BucketExists(ctx, config.Bucket)
		if err != nil {
			return nil, fmt.Errorf("could not check minio bucket %s: %v", config.Bucket, err)
		}
		if !exists {
			slog.Info("configured minio bucket does not exist yet", "name", name, "bucket", config.Bucket)
		}
	}

	return client, nil
}

func getDefaultClient() (*minio.Client, error) {
	if defaultClient == nil {
		return nil, fmt.Errorf("minio client not initialized")
	}
	return defaultClient, nil
}

func getTarget(bucketName string) (*minio.Client, string, error) {
	client, err := getDefaultClient()
	if err != nil {
		return nil, "", err
	}

	resolvedBucket := strings.TrimSpace(bucketName)
	if resolvedBucket == "" {
		resolvedBucket = strings.TrimSpace(defaultBucket)
	}
	if resolvedBucket == "" {
		return nil, "", fmt.Errorf("default minio bucket not configured")
	}
	return client, resolvedBucket, nil
}

func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(strings.ReplaceAll(prefix, "\\", "/"))
	prefix = strings.TrimPrefix(prefix, "/")
	if prefix == "" {
		return ""
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix
}
