ARG VERSION=v1

# ==========================================
# Build stage
# ==========================================
FROM golang:1.26-alpine AS builder
ARG VERSION
ARG TARGETOS
ARG TARGETARCH
WORKDIR /build
RUN apk add --no-cache git ca-certificates
COPY go.work ./
COPY ${VERSION}/ ./${VERSION}
COPY pkg/${VERSION} ./pkg/${VERSION}/

WORKDIR /build/pkg/${VERSION}
RUN go mod download
RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /build/app ./run/server.go

# ==========================================
# Runtime stage
# ==========================================
FROM alpine:3.22
ARG VERSION
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S blobstorage \
    && adduser -S -G blobstorage blobstorage \
    && mkdir -p /app /data \
    && chown -R blobstorage:blobstorage /app /data
WORKDIR /app
COPY --from=builder --chown=blobstorage:blobstorage /build/app /app/blobstorage
ENV S3_HOST=0.0.0.0 \
    S3_PORT=9000 \
    S3_SLUG=/dist \
    S3_STORAGE_PATH=/data
USER blobstorage
EXPOSE 9000
VOLUME ["/data"]
ENTRYPOINT ["/app/blobstorage"]
