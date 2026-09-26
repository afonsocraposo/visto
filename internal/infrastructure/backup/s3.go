package backup

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Config struct {
	Bucket, Region, Endpoint, AccessKeyID, SecretKey, Prefix string
	PathStyle                                                bool
	MaxKeep                                                  int
}
type S3Client struct {
	NewClient  func(context.Context, S3Config) (*s3.Client, error)
	HTTPClient *http.Client
}

func (c S3Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}
func (c S3Client) client(ctx context.Context, cfg S3Config) (*s3.Client, error) {
	if c.NewClient != nil {
		return c.NewClient(ctx, cfg)
	}
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region), config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, "")), config.WithHTTPClient(c.httpClient()))
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.UsePathStyle = cfg.PathStyle
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	}), nil
}
func (c S3Client) Test(ctx context.Context, cfg S3Config) error {
	client, err := c.client(ctx, cfg)
	if err != nil {
		return err
	}
	_, err = client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(cfg.Bucket), Prefix: aws.String(cfg.Prefix), MaxKeys: aws.Int32(1)})
	if err != nil {
		return fmt.Errorf("S3 connection test failed: %w", err)
	}
	nonce := make([]byte, 12)
	if _, err = rand.Read(nonce); err != nil {
		return err
	}
	key := cfg.Prefix + "visto-connection-test-" + hex.EncodeToString(nonce)
	_, err = client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(cfg.Bucket), Key: aws.String(key), Body: bytes.NewReader([]byte("test"))})
	if err != nil {
		return fmt.Errorf("S3 upload test failed: %w", err)
	}
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(cfg.Bucket), Key: aws.String(key)})
	if err != nil {
		return fmt.Errorf("S3 cleanup test failed: %w", err)
	}
	return nil
}
func (c S3Client) Upload(ctx context.Context, cfg S3Config, key, filename string) error {
	client, err := c.client(ctx, cfg)
	if err != nil {
		return err
	}
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(cfg.Bucket), Key: aws.String(key), Body: file, ContentType: aws.String("application/octet-stream")})
	if err != nil {
		return fmt.Errorf("S3 upload failed: %w", err)
	}
	return nil
}
func (c S3Client) Prune(ctx context.Context, cfg S3Config) error {
	client, err := c.client(ctx, cfg)
	if err != nil {
		return err
	}
	var keys []string
	pages := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: aws.String(cfg.Bucket), Prefix: aws.String(cfg.Prefix + backupNamePrefix)})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("list S3 backups: %w", err)
		}
		for _, item := range page.Contents {
			key := aws.ToString(item.Key)
			name := strings.TrimPrefix(key, cfg.Prefix)
			if !strings.HasPrefix(key, cfg.Prefix) || strings.Contains(name, "/") {
				continue
			}
			if _, ok := backupDate(name); ok {
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys)
	for len(keys) > cfg.MaxKeep {
		_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(cfg.Bucket), Key: aws.String(keys[0])})
		if err != nil {
			return fmt.Errorf("delete old S3 backup: %w", err)
		}
		keys = keys[1:]
	}
	return nil
}
