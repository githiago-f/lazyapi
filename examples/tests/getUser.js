import { login, bearer } from "./helpers.js";

test("returns the requested user", () => {
  expect(response).toHaveStatus(200);
  expect(response.json()).toHaveProperty("id", Number(request.params.id));
});

test("unknown users answer 404 with an error body", () => {
  // The mock picks a declared response with the Prefer header.
  const missing = lazyapi.send("getUser", { params: { id: "999" }, headers: { Prefer: "code=404" } });
  expect(missing).toHaveStatus(404);
  expect(missing).toMatchSchema();
  expect(missing.json().error).toContain("not found");
});

test("works with a token from login", () => {
  const res = lazyapi.send("getUser", { params: { id: request.params.id }, headers: bearer(login()) });
  expect(res).toHaveStatus(200);
});
