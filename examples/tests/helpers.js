// Shared helpers for the example tests. Imported with a relative path:
//   import { login, bearer } from "./helpers.js";

/** Logs in and returns the token (chained request through the spec). */
export function login() {
  const res = lazyapi.send("login", { body: { email: "ana@example.com", password: env.get("PASSWORD") ?? "secret" } });
  expect(res).toHaveStatus(200);
  return res.json().token;
}

export const bearer = (token) => ({ Authorization: `Bearer ${token}` });
