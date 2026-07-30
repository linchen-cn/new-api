package common

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

var (
	ossClient      *oss.Client
	ossBucket      *oss.Bucket
	ossInitErr     error
	ossInitOnce    sync.Once
	ossCustomDomain string
)

func ossConfig() (endpoint, accessKeyID, accessKeySecret, bucketName, pathPrefix, customDomain string) {
	return os.Getenv("OSS_ENDPOINT"),
		os.Getenv("OSS_ACCESS_KEY_ID"),
		os.Getenv("OSS_ACCESS_KEY_SECRET"),
		os.Getenv("OSS_BUCKET_NAME"),
		os.Getenv("OSS_PATH_PREFIX"),
		os.Getenv("OSS_CUSTOM_DOMAIN")
}

// IsOSSEnabled checks whether all required OSS env vars are set.
func IsOSSEnabled() bool {
	endpoint, akID, akSecret, bucket, _, _ := ossConfig()
	return endpoint != "" && akID != "" && akSecret != "" && bucket != ""
}

// getOSSBucket lazily initialises the OSS client and bucket.
func getOSSBucket() (*oss.Bucket, error) {
	ossInitOnce.Do(func() {
		endpoint, akID, akSecret, bucket, _, cd := ossConfig()
		if endpoint == "" || akID == "" || akSecret == "" || bucket == "" {
			ossInitErr = fmt.Errorf("OSS configuration incomplete: set OSS_ENDPOINT, OSS_ACCESS_KEY_ID, OSS_ACCESS_KEY_SECRET, OSS_BUCKET_NAME")
			return
		}
		client, err := oss.New(endpoint, akID, akSecret)
		if err != nil {
			ossInitErr = fmt.Errorf("create OSS client: %w", err)
			return
		}
		b, err := client.Bucket(bucket)
		if err != nil {
			ossInitErr = fmt.Errorf("get OSS bucket: %w", err)
			return
		}
		ossClient = client
		ossBucket = b
		ossCustomDomain = cd
	})
	return ossBucket, ossInitErr
}

// UploadToOSS uploads a file to OSS and returns its public URL.
// objectKey is the full path within the bucket (e.g. "playground/images/xxx.png").
func UploadToOSS(reader io.Reader, objectKey string) (string, error) {
	bucket, err := getOSSBucket()
	if err != nil {
		return "", err
	}
	if err := bucket.PutObject(objectKey, reader); err != nil {
		return "", fmt.Errorf("upload to OSS: %w", err)
	}
	return ossPublicURL(objectKey), nil
}

// ossPublicURL builds the public URL for an object.
func ossPublicURL(objectKey string) string {
	if ossCustomDomain != "" {
		return fmt.Sprintf("https://%s/%s", ossCustomDomain, objectKey)
	}
	endpoint, _, _, bucket, _, _ := ossConfig()
	return fmt.Sprintf("https://%s.%s/%s", bucket, endpoint, objectKey)
}

// OSSPathPrefix returns the configured path prefix (defaults to "playground/").
func OSSPathPrefix() string {
	_, _, _, _, prefix, _ := ossConfig()
	if prefix == "" {
		return "playground/"
	}
	if !endsWithSlash(prefix) {
		prefix += "/"
	}
	return prefix
}

func endsWithSlash(s string) bool {
	return len(s) > 0 && s[len(s)-1] == '/'
}
