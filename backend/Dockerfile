# syntax=docker/dockerfile:1
# Targets:
#   runtime (default) - api + migrate, what production runs
#   seed              - demo data loader, never part of the production image:
#                       docker compose --profile tools run --rm seed -history 3
# Base images are pinned by digest; bump the digest together with the tag.

# golang:1.26-alpine
FROM golang@sha256:8ac98ca534ac3f51e1f420a1dd2c15e74c75cfa0f23f3ad27eb5d7236c349a0c AS build
WORKDIR /src
ENV CGO_ENABLED=0
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM build AS build-seed
RUN go build -trimpath -ldflags="-s -w" -o /out/seed ./cmd/seed

# alpine:3.22
FROM alpine@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8 AS base
LABEL app=bugun
# ca-certificates is left unpinned on purpose: it must track CA changes.
# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates \
 && adduser -D -H -u 10001 app \
 && mkdir -p /data/media && chown app:app /data/media
ENV APP_ENV=production HTTP_ADDR=:8080 MEDIA_DIR=/data/media

FROM base AS seed
COPY --from=build-seed /out/seed /usr/local/bin/
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/seed"]

FROM base AS runtime
COPY --from=build /out/api /out/migrate /usr/local/bin/
USER 10001:10001
EXPOSE 8080
ARG GIT_SHA=unknown
LABEL org.opencontainers.image.revision=$GIT_SHA
# busybox wget exits non-zero on any non-2xx, so a 503 (database down) fails.
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s \
  CMD ["wget", "-qO", "/dev/null", "http://127.0.0.1:8080/healthz"]
ENTRYPOINT ["/usr/local/bin/api"]
