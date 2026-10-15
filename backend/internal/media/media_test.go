package media

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"
)

func pngOf(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestProcessDownsizesAndReencodes(t *testing.T) {
	p, err := Process(bytes.NewReader(pngOf(3200, 1600)))
	if err != nil {
		t.Fatal(err)
	}
	if p.Width != 1600 || p.Height != 800 {
		t.Fatalf("size = %dx%d, want 1600x800", p.Width, p.Height)
	}
	if _, err := jpeg.Decode(bytes.NewReader(p.Data)); err != nil {
		t.Fatalf("output is not JPEG: %v", err)
	}
	if !ValidKey(p.Key) || !ValidKey(ThumbKey(p.Key)) {
		t.Fatalf("generated key %q or its thumb key is not valid", p.Key)
	}
	th, err := jpeg.DecodeConfig(bytes.NewReader(p.Thumb))
	if err != nil || th.Width != 480 || th.Height != 240 {
		t.Fatalf("thumb = %dx%d, %v", th.Width, th.Height, err)
	}
}

func TestProcessKeepsSmallImages(t *testing.T) {
	p, err := Process(bytes.NewReader(pngOf(640, 480)))
	if err != nil || p.Width != 640 || p.Height != 480 {
		t.Fatalf("Process = %dx%d, %v", p.Width, p.Height, err)
	}
}

func TestProcessRejectsNonImages(t *testing.T) {
	if _, err := Process(strings.NewReader("<svg onload=alert(1)>")); err != ErrUnsupported {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

func TestProcessRejectsHugeUploads(t *testing.T) {
	if _, err := Process(io.LimitReader(zeros{}, DefaultLimits.MaxBytes+10)); err != ErrTooLarge {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { return len(p), nil }

func TestValidKeyBlocksTraversal(t *testing.T) {
	for _, k := range []string{"../../etc/passwd", "a/b.jpg", "short.jpg", "abcdefghijklmnopqrstuvwxyz.png"} {
		if ValidKey(k) {
			t.Errorf("ValidKey(%q) = true", k)
		}
	}
}

func TestLocalStoreRoundTrip(t *testing.T) {
	s, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := "abcdefghijklmnopqrstuvwxyz012345.jpg"
	if err := s.Put(ctx, key, []byte("data")); err != nil {
		t.Fatal(err)
	}
	rc, err := s.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "data" {
		t.Fatalf("read %q", b)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(ctx, key); err != ErrNotFound {
		t.Fatalf("after delete err = %v, want ErrNotFound", err)
	}
}
