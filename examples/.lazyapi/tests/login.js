// JavaScript/TypeScript test script. F5 runs it against the last response.
// Globals: request, response, env, vars, lazyapi, test, expect
// (run "lazyapi types" for the full API). URL imports are allowed:
//   import { z } from "https://esm.sh/zod@3.23.8";

test("status is 2xx", () => {
  console.log("Logou com sucesso");
  expect(response.status).toBeGreaterThanOrEqual(200);
  expect(response.status).toBeLessThan(300);
});

test("matches the OpenAPI schema", () => {
  expect(response).toMatchSchema();
});
