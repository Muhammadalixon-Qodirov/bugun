package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Config reaches an S3-compatible bucket (AWS S3, MinIO, Cloudflare R2,
// DigitalOcean Spaces, ...). Config: MEDIA_STORAGE=s3 and S3_*.
type S3Config struct {
	// Endpoint is host[:port] or a URL; without a scheme HTTPS is used.
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	// PathStyle addresses the bucket as endpoint/bucket/key instead of
	// bucket.endpoint/key; most self-hosted services need it.
	PathStyle bool
	// PublicBaseURL, when set, serves objects to anyone at base/key: photos
	// of live listings are redirected there instead of streamed by the API.
	PublicBaseURL string
}

// S3Store keeps photos in an S3-compatible bucket, under the same keys as
// the disk store.
type S3Store struct {
	client *minio.Client
	bucket string
	public string
}

// NewS3Store prepares a client for the bucket without contacting it; Check
// does.
func NewS3Store(cfg S3Config) (*S3Store, error) {
	host, secure, err := parseEndpoint(cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	if cfg.Bucket == "" {
		return nil, errors.New("s3: bucket is required")
	}
	lookup := minio.BucketLookupAuto
	if cfg.PathStyle {
		lookup = minio.BucketLookupPath
	}
	c, err := minio.New(host, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       secure,
		Region:       cfg.Region,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, fmt.Errorf("s3: %w", err)
	}
	return &S3Store{client: c, bucket: cfg.Bucket, public: strings.TrimRight(cfg.PublicBaseURL, "/")}, nil
}

func parseEndpoint(ep string) (host string, secure bool, err error) {
	ep = strings.TrimSpace(ep)
	if ep == "" {
		return "", false, errors.New("s3: endpoint is required")
	}
	if !strings.Contains(ep, "://") {
		return ep, true, nil
	}
	u, err := url.Parse(ep)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || (u.Path != "" && u.Path != "/") {
		return "", false, fmt.Errorf("s3: endpoint %q must be host[:port] or http(s)://host[:port]", ep)
	}
	return u.Host, u.Scheme == "https", nil
}

// Check confirms the bucket exists and the credentials work.
func (s *S3Store) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("s3: bucket %q: %w", s.bucket, err)
	}
	if !ok {
		return fmt.Errorf("s3: bucket %q does not exist", s.bucket)
	}
	return nil
}

func (s *S3Store) Put(ctx context.Context, key string, data []byte) error {
	if !ValidKey(key) {
		return ErrNotFound
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType: "image/jpeg",
		// Keys are random and content never changes.
		CacheControl: "public, max-age=31536000, immutable",
	})
	if err != nil {
		return fmt.Errorf("s3 put: %w", err)
	}
	return nil
}

func (s *S3Store) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if !ValidKey(key) {
		return nil, ErrNotFound
	}
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, s3Error(err)
	}
	// GetObject is lazy: Stat makes the request, so a missing key is seen here.
	if _, err := obj.Stat(); err != nil {
		_ = obj.Close()
		return nil, s3Error(err)
	}
	return obj, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	if !ValidKey(key) {
		return ErrNotFound
	}
	// Removing a missing object succeeds, like the disk store.
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
}

// Size returns the stored object's size, or ErrNotFound.
func (s *S3Store) Size(ctx context.Context, key string) (int64, error) {
	if !ValidKey(key) {
		return 0, ErrNotFound
	}
	info, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return 0, s3Error(err)
	}
	return info.Size, nil
}

// PublicURL is where anyone can fetch key directly; ok is false without a
// public base URL.
func (s *S3Store) PublicURL(key string) (string, bool) {
	if s.public == "" || !ValidKey(key) {
		return "", false
	}
	return s.public + "/" + key, true
}

func s3Error(err error) error {
	r := minio.ToErrorResponse(err)
	if r.StatusCode == http.StatusNotFound || r.Code == "NoSuchKey" {
		return ErrNotFound
	}
	return fmt.Errorf("s3: %w", err)
}

// Public is implemented by stores that can serve objects without the API.
type Public interface {
	PublicURL(key string) (string, bool)
}

// Sizer is implemented by stores that can tell whether a key exists without
// reading it.
type Sizer interface {
	Size(ctx context.Context, key string) (int64, error)
}

// NewStore opens the configured photo storage: "disk" (dir) or "s3". An S3
// bucket that is missing or unreachable stops the start, like a disk
// directory that cannot be created.
func NewStore(ctx context.Context, kind, dir string, s3 S3Config) (Store, error) {
	switch kind {
	case "", "disk":
		return NewLocalStore(dir)
	case "s3":
		st, err := NewS3Store(s3)
		if err != nil {
			return nil, err
		}
		if err := st.Check(ctx); err != nil {
			return nil, err
		}
		return st, nil
	}
	return nil, fmt.Errorf("media: unknown storage %q", kind)
}

// SyncResult counts what Sync did.
type SyncResult struct {
	Copied, Skipped int
}

// Sync copies every file of the disk store to dst. A key dst already holds
// with the same size is skipped, so running it again only copies what is
// new; with dryRun nothing is written. Files are only added, never deleted.
func Sync(ctx context.Context, src *LocalStore, dst Store, dryRun bool) (SyncResult, error) {
	var res SyncResult
	sizer, _ := dst.(Sizer)
	err := src.Walk(func(key string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		rc, err := src.Open(ctx, key)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		if sizer != nil {
			n, err := sizer.Size(ctx, key)
			switch {
			case err == nil && n == int64(len(data)):
				res.Skipped++
				return nil
			case err != nil && !errors.Is(err, ErrNotFound):
				return fmt.Errorf("%s: %w", key, err)
			}
		}
		if !dryRun {
			if err := dst.Put(ctx, key, data); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
		}
		res.Copied++
		return nil
	})
	return res, err
}
