# @fehmicorp/s3-proxy

Zero-dependency streaming reverse-proxy middleware for routing Node.js / Express applications to the **Fehmi S3 Object Storage** service (inside Docker or local development) via environment variables.

## Installation

### From GitHub Packages
Add the `@fehmicorp` registry to your project's `.npmrc`:
```ini
@fehmicorp:registry=https://github.com
//://github.com\${GITHUB_TOKEN}
```

Then install:
```bash
npm install @fehmicorp/s3-proxy
```

---

## Configuration (Environment Variables)

| Variable | Default | Description |
| :--- | :--- | :--- |
| `S3_HOST` | `localhost` | Upstream S3 container hostname (e.g., `s3` in Docker, `127.0.0.1` locally) |
| `S3_PORT` | `9000` | Upstream S3 HTTP port |
| `S3_SLUG` | `/s3` | URL path prefix intercepted by the middleware |
| `S3_PROTOCOL` | `http` | Upstream protocol (`http` or `https`) |

---

## Usage

### 1. Express Application
Register `createS3Middleware()` **before** body parsers like `express.json()` so binary file upload streams (`PUT /s3/obj/...`) pass through untouched.

```javascript
const express = require("express");
const { createS3Middleware } = require("@fehmicorp/s3-proxy");

const app = express();

// 1. Proxy /s3/* requests directly to the Go S3 service
app.use(createS3Middleware());

// 2. Your regular JSON body parsers and API routes
app.use(express.json());
app.get("/api/status", (req, res) => res.json({ status: "ok" }));

app.listen(3000, () => console.log("Gateway listening on port 3000"));
```

### 2. Custom Programmatic Options
Override environment variables directly in code if needed:

```javascript
app.use(
  createS3Middleware({
    host: "s3",
    port: 9000,
    slug: "/s3",
    timeout: 300000,
  })
);
```

### 3. Native Node.js `http.createServer`
```javascript
const http = require("node:http");
const { createS3Middleware } = require("@fehmicorp/s3-proxy");

const s3Proxy = createS3Middleware();

http.createServer((req, res) => {
  s3Proxy(req, res, () => {
    res.writeHead(404);
    res.end("Not Found");
  });
}).listen(3000);
```

## License

MIT
