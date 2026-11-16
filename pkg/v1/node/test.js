const test = require("node:test");
const assert = require("node:assert/strict");
const http = require("node:http");
const { createS3Middleware } = require("./index");

test("proxies S3 binary PUT/GET requests and passes through non-matching paths", async () => {
  // 1. Start mock Go S3 upstream server
  const mockS3 = http.createServer((req, res) => {
    const chunks = [];
    req.on("data", (c) => chunks.push(c));
    req.on("end", () => {
      const body = Buffer.concat(chunks).toString("utf8");
      res.writeHead(200, { "Content-Type": "application/json", "X-Mock-Path": req.url });
      res.end(JSON.stringify({ method: req.method, url: req.url, receivedBody: body }));
    });
  });

  await new Promise((resolve) => mockS3.listen(0, "127.0.0.1", resolve));
  const s3Port = mockS3.address().port;

  // 2. Start Gateway server using the middleware
  const middleware = createS3Middleware({
    host: "127.0.0.1",
    port: s3Port,
    slug: "/dist",
  });

  const gateway = http.createServer((req, res) => {
    middleware(req, res, () => {
      res.writeHead(200, { "Content-Type": "text/plain" });
      res.end("handled-by-host-app");
    });
  });

  await new Promise((resolve) => gateway.listen(0, "127.0.0.1", resolve));
  const gwPort = gateway.address().port;

  try {
    // Test A: Matching /dist route streams body + query params to S3
    const s3Res = await fetch(`http://127.0.0.1:${gwPort}/dist/obj/my-bucket/logo.png?uploadId=123`, {
      method: "PUT",
      body: "binary-image-content",
    });
    assert.equal(s3Res.status, 200);
    const json = await s3Res.json();
    assert.equal(json.method, "PUT");
    assert.equal(json.url, "/dist/obj/my-bucket/logo.png?uploadId=123");
    assert.equal(json.receivedBody, "binary-image-content");

    // Test B: Non-matching route falls through to next()
    const appRes = await fetch(`http://127.0.0.1:${gwPort}/api/users`);
    assert.equal(appRes.status, 200);
    assert.equal(await appRes.text(), "handled-by-host-app");
  } finally {
    gateway.close();
    mockS3.close();
  }
});
