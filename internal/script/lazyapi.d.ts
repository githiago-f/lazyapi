// Type definitions for LazyAPI test scripts.
// Generate this file with:  lazyapi types > tests/lazyapi.d.ts
// Reference it from a script with:  /// <reference path="./lazyapi.d.ts" />
//
// Scripts are ES modules (JavaScript or TypeScript). They may import:
//   - relative files:   import { login } from "./helpers.js";
//   - https:// modules: import { z } from "https://esm.sh/zod@3.23.8";
// URL imports are cached and pinned in lazyapi.lock next to the spec.
// Scripts run synchronously in a sandbox without filesystem or process access.
//
// An operation's test script may also export a hook that runs BEFORE the
// request is sent, to change it (auth, signatures, generated bodies):
//
//   export function before(req: BeforeRequest): void | BeforeRequest {
//     req.headers.Authorization = "Bearer " + lazyapi.send("login").json().token;
//   }
//
// The rest of the file (top level and test() calls) runs after the response.
//
// A script may also export after(res), called once the tests are done, even
// when they failed or the top level threw. Use it to clean up what the
// request created (it runs once per script run; res is null in flows):
//
//   export function after(res: LazyResponse | null): void {
//     lazyapi.send("deleteUser", { params: { id: String(res.json().id) } });
//   }
//
// Flows (flows/*.js next to the spec) are scripts without an implicit
// request: they call lazyapi.send() in order and assert with test(). Every
// call made by a flow is checked against the spec automatically.

/** The request that was sent (templates resolved). */
interface LazyRequest {
  readonly method: string;
  /** Full URL with server and path parameters, without query string. */
  readonly url: string;
  /** Path template as written in the spec, e.g. "/users/{id}". */
  readonly path: string;
  /** Header names are lower-cased. */
  readonly headers: Record<string, string>;
  readonly query: Record<string, string>;
  readonly params: Record<string, string>;
  readonly body: string;
  /** Parses the request body as JSON. */
  json<T = any>(): T;
}

/** The request passed to before(req). Change it in place, or return a new object. */
interface BeforeRequest {
  readonly method: string;
  /** Resolved URL (server + path params) at the time before() runs. */
  readonly url: string;
  readonly path: string;
  headers: Record<string, string>;
  query: Record<string, string>;
  params: Record<string, string>;
  /** A string is sent as-is; any other value is JSON-encoded. */
  body: unknown;
}

interface LazyResponse {
  readonly status: number;
  /** e.g. "200 OK" */
  readonly statusText: string;
  /** Header names are lower-cased; repeated headers are joined with ", ". */
  readonly headers: Record<string, string>;
  readonly body: string;
  /** Round-trip time in milliseconds. */
  readonly time: number;
  /** Case-insensitive header lookup; null when missing. */
  header(name: string): string | null;
  /** Parses the body as JSON (cached). Throws if the body is not JSON. */
  json<T = any>(): T;
  text(): string;
}

interface KeyValueStore {
  get(name: string): string | undefined;
  /** Values are converted to strings (objects via JSON). */
  set(name: string, value: unknown): void;
  has(name: string): boolean;
  all(): Record<string, string>;
}

interface SendOptions {
  params?: Record<string, string>;
  query?: Record<string, string>;
  headers?: Record<string, string>;
  /** A string is sent as-is; any other value is JSON-encoded. */
  body?: unknown;
}

interface Matchers {
  /** Inverts the following matcher. */
  readonly not: Matchers;
  toBe(expected: unknown): void;
  /** Deep equality. */
  toEqual(expected: unknown): void;
  toBeTruthy(): void;
  toBeFalsy(): void;
  toBeDefined(): void;
  toBeUndefined(): void;
  toBeNull(): void;
  toBeGreaterThan(n: number): void;
  toBeGreaterThanOrEqual(n: number): void;
  toBeLessThan(n: number): void;
  toBeLessThanOrEqual(n: number): void;
  /** Substring for strings, deep-equal element for arrays. */
  toContain(item: unknown): void;
  toMatch(pattern: RegExp | string): void;
  toHaveLength(n: number): void;
  /** Dotted path ("data.items.0.id") or array of keys. */
  toHaveProperty(path: string | string[], value?: unknown): void;
  /** For responses. */
  toHaveStatus(code: number): void;
  /** For responses. value may be an exact string or a RegExp. */
  toHaveHeader(name: string, value?: string | RegExp): void;
  /**
   * Without arguments on a response: validates status, headers and body
   * against the operation's OpenAPI definition.
   * With a schema: validates the value against that JSON/OpenAPI schema.
   */
  toMatchSchema(schema?: object): void;
}

/** The request being tested. */
declare const request: LazyRequest;
/** The received response; null when no request has been sent yet (TUI). */
declare const response: LazyResponse;
/** Environment variables ({{env.NAME}}). Changes last for the current run. */
declare const env: KeyValueStore;
/** Run variables ({{var.NAME}}), shared by later requests in the same run. */
declare const vars: KeyValueStore;

declare const lazyapi: {
  /**
   * Sends another operation from the same spec and returns its response.
   * target is an operationId or "METHOD /path".
   */
  send(target: string, options?: SendOptions): LazyResponse;
};

/** Registers a test. It fails if fn throws (or returns a rejected promise). */
declare function test(name: string, fn: () => void | Promise<void>): void;
declare function expect(actual: unknown): Matchers;
