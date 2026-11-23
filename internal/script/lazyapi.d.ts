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
// Dynamic values: {{$randomEmail}}, {{$guid}}, … in URLs, params, headers and
// bodies get a new value at every occurrence when the request is sent
// (`lazyapi fake` lists them). Every other function the script exports is a
// helper usable the same way, and replaces a built-in of the same name:
//
//   export function randomSku() { return "SKU-" + faker.int(100, 999); }  // {{$randomSku}}
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
  /** 0-based iteration of a data-driven run (`lazyapi run --data rows.csv`); 0 otherwise. */
  readonly iteration: number;
  /** Number of iterations (rows of --data); 1 otherwise. */
  readonly iterations: number;
  /**
   * Sends another operation from the same spec and returns its response.
   * target is an operationId, "METHOD /path", or "METHOD https://host/users/42?x=1":
   * a URL is mapped to the spec operation it belongs to (path params and query
   * come from the URL) and sent to that host. A URL outside the spec is an error.
   */
  send(target: string, options?: SendOptions): LazyResponse;
};

/** Fake data (gofakeit). Each built-in {{$randomX}} is faker.x(): faker.email(), faker.uuid(), … */
declare const faker: {
  /** Integer in [min, max] (defaults 0, 1000). */
  int(min?: number, max?: number): number;
  /** Number in [min, max) (defaults 0, 1). */
  float(min?: number, max?: number): number;
  /** A random element of list. */
  pick<T>(list: T[]): T;
  /** The value of a built-in dynamic variable: faker.get("randomEmail"). */
  get(name: string): string;
  guid(): string; timestamp(): string; isoTimestamp(): string; uuid(): string;
  alphaNumeric(): string; boolean(): string; color(): string; hexColor(): string; abbreviation(): string;
  ip(): string; ipv6(): string; macAddress(): string; password(): string; locale(): string; userAgent(): string;
  protocol(): string; semver(): string; email(): string; exampleEmail(): string; userName(): string; url(): string;
  domainName(): string; domainSuffix(): string; domainWord(): string;
  firstName(): string; lastName(): string; fullName(): string; namePrefix(): string; nameSuffix(): string;
  jobArea(): string; jobDescriptor(): string; jobTitle(): string; jobType(): string; gender(): string; age(): string;
  phoneNumber(): string; phoneNumberExt(): string; city(): string; streetName(): string; streetAddress(): string;
  state(): string; zipCode(): string; country(): string; countryCode(): string; latitude(): string; longitude(): string;
  timeZone(): string; avatarImage(): string; imageUrl(): string;
  companyName(): string; companySuffix(): string; bs(): string; catchPhrase(): string; price(): string;
  currencyCode(): string; currencyName(): string; bitcoin(): string; bankAccount(): string; bankAccountName(): string;
  creditCardMask(): string; creditCardNumber(): string; transactionType(): string;
  dateFuture(): string; datePast(): string; dateRecent(): string; weekday(): string; month(): string;
  fileName(): string; fileExt(): string; mimeType(): string;
  productName(): string; product(): string; productMaterial(): string; productAdjective(): string; department(): string;
  noun(): string; verb(): string; ingverb(): string; adjective(): string; word(): string; words(): string; phrase(): string;
  loremWord(): string; loremWords(): string; loremSentence(): string; loremSentences(): string;
  loremParagraph(): string; loremSlug(): string; loremText(): string;
};

/** Registers a test. It fails if fn throws (or returns a rejected promise). */
declare function test(name: string, fn: () => void | Promise<void>): void;
declare function expect(actual: unknown): Matchers;
