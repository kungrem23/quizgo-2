package s3

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

// Config supports AWS S3 and compatible stores. BrowserEndpoint is used only
// when signing URLs, for example when Docker uses an internal upload endpoint.
type Config struct {
	Bucket          string
	Region          string
	Endpoint        string
	BrowserEndpoint string
	ForcePathStyle  bool
	URLTTL          time.Duration
}

type Store struct {
	client    *awss3.Client
	presigner *awss3.PresignClient
	bucket    string
	ttl       time.Duration
}

func New(ctx context.Context, cfg Config) (*Store, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("S3_BUCKET is required")
	}
	if cfg.Region == "" {
		return nil, fmt.Errorf("S3_REGION is required")
	}
	if cfg.URLTTL < time.Minute || cfg.URLTTL > 7*24*time.Hour {
		return nil, fmt.Errorf("S3_URL_TTL must be between 1m and 168h")
	}
	for _, endpoint := range []string{cfg.Endpoint, cfg.BrowserEndpoint} {
		if endpoint == "" {
			continue
		}
		u, err := url.Parse(endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return nil, fmt.Errorf("S3 endpoints must be absolute HTTP(S) origins without credentials or paths")
		}
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("load AWS configuration: %w", err)
	}
	newClient := func(endpoint string) *awss3.Client {
		return awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
			o.UsePathStyle = cfg.ForcePathStyle
			if endpoint != "" {
				o.BaseEndpoint = aws.String(strings.TrimRight(endpoint, "/"))
			}
		})
	}
	client := newClient(cfg.Endpoint)
	signerClient := client
	if cfg.BrowserEndpoint != "" {
		signerClient = newClient(cfg.BrowserEndpoint)
	}
	return &Store{client: client, presigner: awss3.NewPresignClient(signerClient), bucket: cfg.Bucket, ttl: cfg.URLTTL}, nil
}

// Object keys are derived from the immutable image ID; URLs never enter the DB.
func objectKey(id string) string { return "images/" + id }

func (s *Store) Put(ctx context.Context, id, contentType string, content []byte) error {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	_, err := s.client.PutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(objectKey(id)),
		Body: bytes.NewReader(content), ContentType: aws.String(contentType),
		CacheControl: aws.String("private, max-age=300"),
	})
	return err
}

func (s *Store) URL(ctx context.Context, id string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result, err := s.presigner.PresignGetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(objectKey(id)),
	}, func(o *awss3.PresignOptions) { o.Expires = s.ttl })
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (s *Store) Delete(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err := s.client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(objectKey(id))})
	return err
}
