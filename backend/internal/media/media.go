// Package media validates, normalises and stores listing photos.
package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // decoders registered for image.Decode
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"bugun/internal/secure"
)

// Limits bound an upload (config: MEDIA_MAX_UPLOAD_BYTES, MEDIA_MAX_PIXELS).
type Limits struct {
	MaxBytes  int64 // file size, checked before decoding
	MaxPixels int   // width × height, checked before decoding the pixels
}

// DefaultLimits: 10 MB and 40 megapixels.
var DefaultLimits = Limits{MaxBytes: 10 << 20, MaxPixels: 40_000_000}

const (
	maxSide      = 1600
	thumbSide    = 480
	jpegQuality  = 82
	thumbQuality = 78
)

var (
	ErrTooLarge    = errors.New("image too large")
	ErrUnsupported = errors.New("unsupported image")
	ErrNotFound    = errors.New("media not found")
	keyPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{20,64}\.jpg$`)
)

// Store keeps processed images. The local disk store is used in development;
// an S3-compatible store can implement the same interface for production.
type Store interface {
	Put(ctx context.Context, key string, data []byte) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// Photo is a processed image ready to be stored, with a small copy for cards.
type Photo struct {
	Key           string
	Data          []byte
	Thumb         []byte
	Width, Height int
}

// ThumbKey is where the card-size copy of key is stored.
func ThumbKey(key string) string { return strings.TrimSuffix(key, ".jpg") + "_t.jpg" }

// Save stores the photo and its thumbnail under p.Key.
func Save(ctx context.Context, s Store, p Photo) error {
	if err := s.Put(ctx, p.Key, p.Data); err != nil {
		return err
	}
	if len(p.Thumb) > 0 {
		return s.Put(ctx, ThumbKey(p.Key), p.Thumb)
	}
	return nil
}

// Remove deletes the photo and its thumbnail.
func Remove(ctx context.Context, s Store, key string) error {
	_ = s.Delete(ctx, ThumbKey(key))
	return s.Delete(ctx, key)
}

// Process is Limits.Process under the default limits.
func Process(r io.Reader) (Photo, error) { return DefaultLimits.Process(r) }

// Process decodes an upload, downsizes it to at most 1600px on the long side
// and re-encodes it as JPEG. Re-encoding also drops EXIF, including GPS.
func (l Limits) Process(r io.Reader) (Photo, error) {
	raw, err := io.ReadAll(io.LimitReader(r, l.MaxBytes+1))
	if err != nil {
		return Photo{}, err
	}
	if int64(len(raw)) > l.MaxBytes {
		return Photo{}, ErrTooLarge
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return Photo{}, ErrUnsupported
	}
	if cfg.Width*cfg.Height > l.MaxPixels || cfg.Width < 1 || cfg.Height < 1 {
		return Photo{}, ErrTooLarge
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return Photo{}, ErrUnsupported
	}

	full, w, h, err := encode(src, maxSide, jpegQuality)
	if err != nil {
		return Photo{}, err
	}
	thumb, _, _, err := encode(src, thumbSide, thumbQuality)
	if err != nil {
		return Photo{}, err
	}
	return Photo{Key: secure.Token(24) + ".jpg", Data: full, Thumb: thumb, Width: w, Height: h}, nil
}

// encode scales src to fit side x side (never up) and returns JPEG bytes.
func encode(src image.Image, side, quality int) ([]byte, int, int, error) {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if w > side || h > side {
		if w >= h {
			h = max(1, h*side/w)
			w = side
		} else {
			w = max(1, w*side/h)
			h = side
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	// Transparent PNGs get a white background instead of black.
	draw.Draw(dst, dst.Bounds(), image.White, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: quality}); err != nil {
		return nil, 0, 0, fmt.Errorf("encode jpeg: %w", err)
	}
	return buf.Bytes(), w, h, nil
}

// ValidKey guards against path traversal: keys are generated, never user input.
func ValidKey(key string) bool { return keyPattern.MatchString(key) }

// LocalStore keeps files in a directory outside the repository.
type LocalStore struct{ dir string }

func NewLocalStore(dir string) (*LocalStore, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("media dir: %w", err)
	}
	return &LocalStore{dir: dir}, nil
}

func (s *LocalStore) path(key string) (string, error) {
	if !ValidKey(key) {
		return "", ErrNotFound
	}
	return filepath.Join(s.dir, key[:2], key), nil
}

func (s *LocalStore) Put(_ context.Context, key string, data []byte) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (s *LocalStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	p, err := s.path(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	p, err := s.path(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Walk calls fn with the key of every stored file, photos and thumbnails.
func (s *LocalStore) Walk(fn func(key string) error) error {
	return filepath.WalkDir(s.dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !ValidKey(d.Name()) {
			return nil // temporary files and anything else left in the directory
		}
		return fn(d.Name())
	})
}
