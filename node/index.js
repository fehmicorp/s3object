const http = require("node:http");
const https = require("node:https");

function normalizeSlug(slug) {
  if (!slug || slug === "/") return "";
  let s = slug.trim();
  if (!s.startsWith("/")) s = "/" + s;
  return s.replace(/\/+$/, "");
}

function createS3Middleware(options = {}) {
  const host = options.host || process.env.S3_HOST || "localhost";
  const port = Number(options.port || process.env.S3_PORT || 9000);
  const protocol = (options.protocol || process.env.S3_PROTOCOL || "http").replace(":", "");
  const slug = normalizeSlug(options.slug !== undefined ? options.slug : (process.env.S3_SLUG ?? "/dist"));
  const timeout = Number(options.timeout || 300000); // 5 minutes for large uploads

  const transport = protocol === "https" ? https : http;

  return function s3ProxyMiddleware(req, res, next) {
    // Support both Express mounted routes (app.use('/dist', middleware)) and raw http.createServer
    const incomingUrl = req.originalUrl || req.url || "/";

    // Only intercept requests matching S3_SLUG when used globally
    if (slug && !incomingUrl.startsWith(slug + "/") && incomingUrl !== slug && !incomingUrl.startsWith(slug + "?")) {
      if (typeof next === "function") return next();
      res.statusCode = 404;
      res.end("Not Found");
      return;
    }

    // Preserve query strings (?uploadId=...&partNumber=1&X-Fehmi-Signature=...)
    const targetPath = incomingUrl;

    const headers = {
      ...req.headers,
      host: `${host}:${port}`,
      "x-forwarded-host": req.headers.host || "",
      "x-forwarded-proto": req.socket?.encrypted ? "https" : "http",
    };

    const proxyReq = transport.request(
      {
        hostname: host,
        port,
        path: targetPath,
        method: req.method,
        headers,
        timeout,
      },
      (proxyRes) => {
        res.writeHead(proxyRes.statusCode || 502, proxyRes.headers);
        proxyRes.pipe(res, { end: true });
      }
    );

    proxyReq.on("timeout", () => {
      proxyReq.destroy(new Error("S3 upstream request timed out"));
    });

    proxyReq.on("error", (err) => {
      if (res.headersSent) {
        res.end();
        return;
      }
      res.writeHead(502, { "Content-Type": "application/json" });
      res.end(
        JSON.stringify({
          error: "S3UpstreamUnavailable",
          message: `Failed to reach S3 backend at ${protocol}://${host}:${port}${slug}`,
          detail: err.message,
        })
      );
    });

    // Abort upstream request if client disconnects mid-upload
    req.on("aborted", () => {
      proxyReq.destroy();
    });

    // Stream incoming request body directly to Go S3 server (supports multi-GB files)
    req.pipe(proxyReq, { end: true });
  };
}

module.exports = { createS3Middleware };