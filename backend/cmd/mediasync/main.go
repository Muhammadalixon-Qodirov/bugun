// Command mediasync copies listing photos from the local media directory to
// the S3-compatible bucket, before switching the API to MEDIA_STORAGE=s3.
//
//	mediasync            copy files the bucket does not have yet
//	mediasync -dry-run   only count them
//
// It reads MEDIA_DIR and the S3_* settings from the environment (or .env in
// development). Files already in the bucket with the same size are skipped,
// so it can run again after new uploads; nothing is ever deleted.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"

	"bugun/internal/media"
)

type settings struct {
	MediaDir       string `env:"MEDIA_DIR" envDefault:"data/media"`
	S3Endpoint     string `env:"S3_ENDPOINT,required"`
	S3Region       string `env:"S3_REGION"`
	S3Bucket       string `env:"S3_BUCKET,required"`
	S3AccessKey    string `env:"S3_ACCESS_KEY,required"`
	S3SecretKey    string `env:"S3_SECRET_KEY,required"`
	S3UsePathStyle bool   `env:"S3_USE_PATH_STYLE"`
}

func main() {
	if err := run(); err != nil {
		slog.Error("mediasync", "err", err)
		os.Exit(1)
	}
}

func run() error {
	dryRun := flag.Bool("dry-run", false, "count the files to copy without copying them")
	flag.Parse()
	_ = godotenv.Load()
	var s settings
	if err := env.Parse(&s); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if _, err := os.Stat(s.MediaDir); err != nil {
		return err
	}
	src, err := media.NewLocalStore(s.MediaDir)
	if err != nil {
		return err
	}
	dst, err := media.NewS3Store(media.S3Config{
		Endpoint: s.S3Endpoint, Region: s.S3Region, Bucket: s.S3Bucket,
		AccessKey: s.S3AccessKey, SecretKey: s.S3SecretKey, PathStyle: s.S3UsePathStyle,
	})
	if err != nil {
		return err
	}
	if err := dst.Check(ctx); err != nil {
		return err
	}
	res, err := media.Sync(ctx, src, dst, *dryRun)
	slog.Info("mediasync", "copied", res.Copied, "already_there", res.Skipped, "dry_run", *dryRun)
	return err
}
