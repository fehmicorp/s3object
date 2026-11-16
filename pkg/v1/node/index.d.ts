import { IncomingMessage, ServerResponse } from "node:http";

export interface S3ProxyOptions {
  host?: string;
  port?: number;
  slug?: string;
  protocol?: "http" | "https";
  timeout?: number;
}

export type S3Middleware = (
  req: IncomingMessage & { originalUrl?: string },
  res: ServerResponse,
  next?: (err?: unknown) => void
) => void;

export function createS3Middleware(options?: S3ProxyOptions): S3Middleware;