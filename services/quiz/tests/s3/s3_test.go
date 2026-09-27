package s3_test

import . "github.com/kungrem23/quizgo/services/quiz/internal/s3"

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

func TestSDKUploadsAndDeletesAndSignsBrowserEndpoint(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test-access")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("AWS_SESSION_TOKEN", "")
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.URL.Path != "/quizgo/images/image-1" || !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Errorf("unsigned or wrong request: %s", r.URL.Path)
		}
		if r.Method == http.MethodPut {
			data, _ := io.ReadAll(r.Body)
			if string(data) != "png data" || r.Header.Get("Content-Type") != "image/png" {
				t.Error("upload changed image")
			}
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	store, err := New(context.Background(), Config{Bucket: "quizgo", Region: "us-east-1", Endpoint: server.URL, BrowserEndpoint: "https://images.example.test", ForcePathStyle: true, URLTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Put(context.Background(), "image-1", "image/png", []byte("png data")); err != nil {
		t.Fatal(err)
	}
	link, err := store.URL(context.Background(), "image-1")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(link)
	if u.Host != "images.example.test" || u.Path != "/quizgo/images/image-1" || u.Query().Get("X-Amz-Expires") != "3600" || u.Query().Get("X-Amz-Signature") == "" {
		t.Fatal("incorrect browser URL")
	}
	if err = store.Delete(context.Background(), "image-1"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(methods, ",") != "PUT,DELETE" {
		t.Fatal(methods)
	}
}

func TestRejectInvalidStorageConfig(t *testing.T) {
	for _, endpoint := range []string{"s3.internal", "ftp://s3.internal", "http://user:password@host", "https://host/bucket", "https://host?key=secret"} {
		_, err := New(context.Background(), Config{Bucket: "quizgo", Region: "us-east-1", Endpoint: endpoint, URLTTL: time.Hour})
		if err == nil {
			t.Errorf("accepted %s", endpoint)
		}
	}
}

// Optional real S3-compatible integration. Uses only a new random test bucket.
func TestS3Integration(t *testing.T) {
	endpoint := os.Getenv("QUIZ_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("QUIZ_TEST_S3_ENDPOINT is unset")
	}
	ctx := context.Background()
	bucket := "quizgo-test-" + uuid.NewString()
	awsSettings, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion("us-east-1"))
	if err != nil {
		t.Fatal(err)
	}
	admin := awss3.NewFromConfig(awsSettings, func(options *awss3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
	store, err := New(ctx, Config{Bucket: bucket, Region: "us-east-1", Endpoint: endpoint, ForcePathStyle: true, URLTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = admin.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatal(err)
	}
	defer admin.DeleteBucket(ctx, &awss3.DeleteBucketInput{Bucket: aws.String(bucket)})
	id := uuid.NewString()
	if err = store.Put(ctx, id, "image/png", []byte("test object bytes")); err != nil {
		t.Fatal(err)
	}
	defer store.Delete(ctx, id)
	link, err := store.URL(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Get(link)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(body) != "test object bytes" || res.Header.Get("Content-Type") != "image/png" {
		t.Fatalf("download status=%d", res.StatusCode)
	}
	if err = store.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	res, err = http.Get(link)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("deleted object status=%d", res.StatusCode)
	}
}
