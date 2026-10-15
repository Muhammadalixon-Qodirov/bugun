package media

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

// fakeS3 runs an in-memory S3 service with one bucket and returns a store on it.
func fakeS3(t *testing.T, public string) *S3Store {
	t.Helper()
	backend := s3mem.New()
	if err := backend.CreateBucket("photos"); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(gofakes3.New(backend).Server())
	t.Cleanup(srv.Close)
	s, err := NewS3Store(S3Config{Endpoint: srv.URL, Region: "us-east-1", Bucket: "photos", AccessKey: "test", SecretKey: "test",
		PathStyle: true, PublicBaseURL: public})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Check(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestS3Store(t *testing.T) {
	ctx := context.Background()
	s := fakeS3(t, "")
	key := "aaaaaaaaaaaaaaaaaaaaaaaa.jpg"

	if _, err := s.Open(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing key: %v, want ErrNotFound", err)
	}
	if err := Save(ctx, s, Photo{Key: key, Data: []byte("full"), Thumb: []byte("thumb")}); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{key: "full", ThumbKey(key): "thumb"} {
		rc, err := s.Open(ctx, k)
		if err != nil {
			t.Fatalf("open %s: %v", k, err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		if string(b) != want {
			t.Errorf("%s = %q, want %q", k, b, want)
		}
	}
	if n, err := s.Size(ctx, key); err != nil || n != 4 {
		t.Errorf("Size = %d, %v", n, err)
	}
	if _, ok := s.PublicURL(key); ok {
		t.Error("PublicURL without a base URL")
	}
	if err := Remove(ctx, s, key); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(ctx, ThumbKey(key)); !errors.Is(err, ErrNotFound) {
		t.Errorf("after Remove: %v, want ErrNotFound", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Errorf("deleting twice: %v", err)
	}
	for _, bad := range []string{"../x.jpg", "short.jpg", ""} {
		if err := s.Put(ctx, bad, []byte("x")); !errors.Is(err, ErrNotFound) {
			t.Errorf("Put(%q) = %v, want ErrNotFound", bad, err)
		}
	}

	pub := fakeS3(t, "https://cdn.example.com/photos/")
	if u, ok := pub.PublicURL(key); !ok || u != "https://cdn.example.com/photos/"+key {
		t.Errorf("PublicURL = %q, %v", u, ok)
	}
}

func TestS3StoreMissingBucket(t *testing.T) {
	srv := httptest.NewServer(gofakes3.New(s3mem.New()).Server())
	defer srv.Close()
	s, err := NewS3Store(S3Config{Endpoint: srv.URL, Bucket: "nope", AccessKey: "a", SecretKey: "b", PathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Check(context.Background()); err == nil {
		t.Error("Check passed for a missing bucket")
	}
	if _, err := NewStore(context.Background(), "s3", "", S3Config{Endpoint: srv.URL, Bucket: "nope", AccessKey: "a", SecretKey: "b", PathStyle: true}); err == nil {
		t.Error("NewStore accepted a missing bucket")
	}
}

func TestParseEndpoint(t *testing.T) {
	for in, want := range map[string]struct {
		host   string
		secure bool
	}{
		"s3.example.com":         {"s3.example.com", true},
		"https://s3.example.com": {"s3.example.com", true},
		"http://127.0.0.1:9000":  {"127.0.0.1:9000", false},
		"http://minio:9000/":     {"minio:9000", false},
	} {
		host, secure, err := parseEndpoint(in)
		if err != nil || host != want.host || secure != want.secure {
			t.Errorf("%q = %q %v %v, want %+v", in, host, secure, err, want)
		}
	}
	for _, bad := range []string{"", "ftp://x", "https://x/bucket"} {
		if _, _, err := parseEndpoint(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestSyncCopiesOnce(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	disk, err := NewLocalStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"bbbbbbbbbbbbbbbbbbbbbbbb.jpg", "cccccccccccccccccccccccc.jpg"} {
		if err := Save(ctx, disk, Photo{Key: k, Data: []byte("full " + k), Thumb: []byte("thumb")}); err != nil {
			t.Fatal(err)
		}
	}
	// Leftovers that are not photos stay behind.
	if err := os.WriteFile(filepath.Join(dir, "bb", "x.jpg.tmp"), []byte("tmp"), 0o600); err != nil {
		t.Fatal(err)
	}
	s3 := fakeS3(t, "")

	if res, err := Sync(ctx, disk, s3, true); err != nil || res.Copied != 4 {
		t.Fatalf("dry run = %+v, %v", res, err)
	}
	if _, err := s3.Size(ctx, "bbbbbbbbbbbbbbbbbbbbbbbb.jpg"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("dry run wrote: %v", err)
	}
	if res, err := Sync(ctx, disk, s3, false); err != nil || res.Copied != 4 || res.Skipped != 0 {
		t.Fatalf("first sync = %+v, %v", res, err)
	}
	if res, err := Sync(ctx, disk, s3, false); err != nil || res.Copied != 0 || res.Skipped != 4 {
		t.Fatalf("second sync = %+v, %v", res, err)
	}
	rc, err := s3.Open(ctx, ThumbKey("cccccccccccccccccccccccc.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	if b, _ := io.ReadAll(rc); string(b) != "thumb" {
		t.Errorf("copied thumbnail = %q", b)
	}
}
