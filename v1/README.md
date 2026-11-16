# Fehmi S3 Object Storage Server

A lightweight, self-hosted S3-compatible object and asset storage API built in Go with local filesystem storage and embedded SQLite state.

## Quick Start

### 1. Prerequisites
* **Go 1.26+** (required for enhanced `net/http.ServeMux` method and wildcard routing)
* Local storage directory with read/write permissions

### 2. Configuration (`config.yaml`)
```yaml
app:
  name: "fehmi-s3"
  version: "0.1.0"
  environment: "development"

server:
  host: "0.0.0.0"
  port: 9000
  initial_slug: "/dist"
  read_timeout: 300s
  write_timeout: 300s
  idle_timeout: 120s
  max_header_bytes: 1048576

storage:
  driver: "filesystem"
  path: "./data/persistent"
  staging_path: "./data/staging"
  max_upload_size: 10737418240 # 10 GB
  chunk_size: 8388608          # 8 MB

security:
  checksum_algorithm: "sha256"
  presigned_url_expiry: 400s
```

### 3. Run the Server
```bash
go run ./cmd/server
```
Default Base URL: `http://localhost:9000/dist`

---

## API Endpoints & Usage Guide

### 1. Health Check

#### Check Server Status
* **Method:** `GET`
* **Endpoint:** `/health`
```bash
curl -X GET http://localhost:9000/dist/health
```
**Response (`200 OK`):**
```json
{
  "app": "fehmi-s3",
  "status": "alive",
  "storage": "./data/persistent",
  "version": "0.1.0"
}
```

---

### 2. Bucket Management

#### Create Bucket
* **Method:** `PUT`
* **Endpoint:** `/create/{bucket}`
```bash
curl -X PUT http://localhost:9000/dist/create/my-assets
```

#### List All Buckets
* **Method:** `GET`
* **Endpoint:** `/list`
```bash
curl -X GET http://localhost:9000/dist/list
```

#### Check Bucket Exists (`HEAD`)
* **Method:** `HEAD`
* **Endpoint:** `/fetch/{bucket}`
```bash
curl -I http://localhost:9000/dist/fetch/my-assets
```

#### Delete Empty Bucket
* **Method:** `DELETE`
* **Endpoint:** `/delete/{bucket}`
```bash
curl -X DELETE http://localhost:9000/dist/delete/my-assets
```
*(Returns `409 Conflict` if the bucket still contains files.)*

---

### 3. Object / Asset Operations

#### Upload Object (Binary Stream)
* **Method:** `PUT`
* **Endpoint:** `/obj/{bucket}/{key...}`
```bash
curl -X PUT http://localhost:9000/dist/obj/my-assets/images/2026/logo.png \
  --data-binary "@./logo.png"
```
**Response (`200 OK`):**
```json
{
  "bucket": "my-assets",
  "etag": "\"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\"",
  "key": "images/2026/logo.png",
  "size": 24512
}
```

#### List Objects in Bucket
* **Method:** `GET`
* **Endpoint:** `/objs/{bucket}`
* **Query Params:** `prefix` (optional), `max-keys` (optional, default `1000`)
```bash
curl -X GET "http://localhost:9000/dist/objs/my-assets?prefix=images/2026/&max-keys=100"
```

#### Download Object (Supports HTTP Range / Resume)
* **Method:** `GET`
* **Endpoint:** `/obj/{bucket}/{key...}`
```bash
# Full download
curl -X GET http://localhost:9000/dist/obj/my-assets/images/2026/logo.png -o logo.png

# Partial / Resumable byte range download
curl -X GET http://localhost:9000/dist/obj/my-assets/images/2026/logo.png \
  -H "Range: bytes=0-1023" -o logo.part
```

#### Check Object Metadata (`HEAD`)
* **Method:** `HEAD`
* **Endpoint:** `/obj/{bucket}/{key...}`
```bash
curl -I http://localhost:9000/dist/obj/my-assets/images/2026/logo.png
```

#### Delete Object
* **Method:** `DELETE`
* **Endpoint:** `/obj/{bucket}/{key...}`
```bash
curl -X DELETE http://localhost:9000/dist/obj/my-assets/images/2026/logo.png
```

---

### 4. Multipart Uploads (Large Files & Agent Sync)

#### Step 1: Initialize Multipart Session
* **Method:** `POST`
* **Endpoint:** `/begin/{bucket}/{key...}`
```bash
curl -X POST http://localhost:9000/dist/begin/my-assets/videos/demo.mp4
```
**Response (`200 OK`):**
```json
{
  "bucket": "my-assets",
  "key": "videos/demo.mp4",
  "upload_id": "a1b2c3d4e5f60718293a4b5c6d7e8f90"
}
```

#### Step 2: Upload Chunks (`partNumber` 1–10000)
* **Method:** `PUT`
* **Endpoint:** `/part/{bucket}/{key...}?uploadId={id}&partNumber={num}`
```bash
curl -X PUT "http://localhost:9000/dist/part/my-assets/videos/demo.mp4?uploadId=a1b2c3d4e5f60718293a4b5c6d7e8f90&partNumber=1" \
  --data-binary "@./part1.bin"

curl -X PUT "http://localhost:9000/dist/part/my-assets/videos/demo.mp4?uploadId=a1b2c3d4e5f60718293a4b5c6d7e8f90&partNumber=2" \
  --data-binary "@./part2.bin"
```

#### Step 3: Complete & Assemble File
* **Method:** `POST`
* **Endpoint:** `/done/{bucket}/{key...}?uploadId={id}`
```bash
curl -X POST "http://localhost:9000/dist/done/my-assets/videos/demo.mp4?uploadId=a1b2c3d4e5f60718293a4b5c6d7e8f90" \
  -H "Content-Type: application/json" \
  -d '{
    "parts": [
      {"part_number": 1, "etag": "<ETAG_FROM_PART_1>"},
      {"part_number": 2, "etag": "<ETAG_FROM_PART_2>"}
    ]
  }'
```

#### Step 4: Abort & Clean Up Staged Chunks (Optional)
* **Method:** `DELETE`
* **Endpoint:** `/end/{bucket}/{key...}?uploadId={id}`
```bash
curl -X DELETE "http://localhost:9000/dist/end/my-assets/videos/demo.mp4?uploadId=a1b2c3d4e5f60718293a4b5c6d7e8f90"
```

---

### 5. Presigned URLs & Asset Search

#### Generate Presigned URL
* **Method:** `POST`
* **Endpoint:** `/sign`
```bash
curl -X POST http://localhost:9000/dist/sign \
  -H "Content-Type: application/json" \
  -d '{
    "bucket": "my-assets",
    "key": "images/2026/logo.png",
    "method": "GET",
    "expiry_seconds": 600
  }'
```
**Response (`200 OK`):**
```json
{
  "bucket": "my-assets",
  "expires_at": "2026-11-16T11:05:00Z",
  "key": "images/2026/logo.png",
  "method": "GET",
  "url": "http://localhost:9000/dist/obj/my-assets/images/2026/logo.png?X-Fehmi-Expires=1794827100&X-Fehmi-Signature=..."
}
```

#### Search Assets Across Buckets
* **Method:** `GET`
* **Endpoint:** `/find`
* **Query Params:** `q` (required), `bucket` (optional filter), `limit` (optional, default `50`, max `500`)
```bash
# Search across all buckets
curl -X GET "http://localhost:9000/dist/find?q=logo&limit=20"

# Search within a specific bucket
curl -X GET "http://localhost:9000/dist/find?q=logo&bucket=my-assets"
```

---

## Error Codes

| HTTP Status | Error Code | Description |
| :--- | :--- | :--- |
| `400 Bad Request` | `InvalidBucketName` / `InvalidObjectKey` | Invalid characters or directory traversal (`..`) detected. |
| `403 Forbidden` | `PresignedURLExpired` / `SignatureDoesNotMatch` | Presigned link expired or tampered with. |
| `404 Not Found` | `NoSuchBucket` / `NoSuchKey` / `NoSuchUpload` | Bucket, object, or multipart session does not exist. |
| `409 Conflict` | `BucketAlreadyOwnedByYou` / `BucketNotEmpty` | Bucket already exists or cannot be deleted while non-empty. |
