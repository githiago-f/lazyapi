import { login, bearer } from "./helpers.js";

// Runs before the request is sent: authenticate it.
export function before(req) {
  Object.assign(req.headers, bearer(login()));
}

test("returns the requested user", () => {
  expect(response).toHaveStatus(200);
  expect(response.json()).toHaveProperty("id", Number(request.params.id));
});

test("the request was authenticated by before()", () => {
  expect(request.headers.authorization).toMatch(/^Bearer /);
});

test("unknown users answer 404 with an error body", () => {
  // The mock picks a declared response with the Prefer header.
  const missing = lazyapi.send("getUser", { params: { id: "999" }, headers: { Prefer: "code=404" } });
  expect(missing).toHaveStatus(404);
  expect(missing).toMatchSchema();
  expect(missing.json().error).toContain("not found");
});
