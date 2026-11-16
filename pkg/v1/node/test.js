const test = require("node:test");
const assert = require("node:assert/strict");
const http = require("node:http");
const { setTimeout: delay } = require("node:timers/promises");
const { createS3Middleware } = require("./index");

test("checks /s3 health, creates bucket dir on real S3 server, lists it, waits 5s, and removes it", async () => {
  const s3Host = process.env.S3_HOST || "127.0.0.1";
  const s3Port = Number(process.env.S3_PORT || 9000);
  const s3Slug = process.env.S3_SLUG || "/s3"; // Changed from "/dist" to "/s3"

  // 1. Start Gateway server using the middleware pointing to /s3
  const middleware = createS3Middleware({
    host: s3Host,
    port: s3Port,
    slug: s3Slug,
  });

  const gateway = http.createServer((req, res) => {
    middleware(req, res, () => {
      res.writeHead(200, { "Content-Type": "text/plain" });
      res.end("handled-by-host-app");
    });
  });

  await new Promise((resolve) => gateway.listen(0, "127.0.0.1", resolve));
  const gwPort = 9000;
  const baseUrl = `http://127.0.0.1:${gwPort}${s3Slug}`;
  const bucketName = `test-dir-${Date.now()}`;

  try {
    // Step 1: Check health at http://127.0.0.1:<gwPort>/s3
    const healthRes = await fetch(baseUrl);
    assert.equal(healthRes.status, 200, `Expected 200 from ${baseUrl}, got ${healthRes.status}`);
    const healthJson = await healthRes.json();
    console.log("Health check (/s3) passed:", healthJson);

    // Step 2: Create new bucket directory on real storage
    const createRes = await fetch(`${baseUrl}/create/${bucketName}`, { method: "PUT" });
    assert.equal(createRes.status, 200);
    const createJson = await createRes.json();
    console.log("Created dir:", createJson);

    // Step 3: List directories and verify it exists
    const listRes = await fetch(`${baseUrl}/list`);
    assert.equal(listRes.status, 200);
    const listJson = await listRes.json();
    console.log("Listed dirs:", listJson);
    assert.ok(
      listJson.buckets.some((b) => b.name === bucketName),
      `Bucket ${bucketName} was not found in /s3/list response`
    );

    // Step 4: Wait 5 seconds
    console.log(`Waiting 5 seconds before removing '${bucketName}'...`);
    await delay(5000);

    // Step 5: Remove the bucket directory
    const deleteRes = await fetch(`${baseUrl}/delete/${bucketName}`, { method: "DELETE" });
    assert.equal(deleteRes.status, 204);
    console.log(`Removed dir '${bucketName}'`);

    // Step 6: Verify directory is gone
    const listAfterRes = await fetch(`${baseUrl}/list`);
    const listAfterJson = await listAfterRes.json();
    assert.equal(
      listAfterJson.buckets.some((b) => b.name === bucketName),
      false,
      `Bucket ${bucketName} still exists after delete`
    );
  } finally {
    gateway.close();
  }
});
