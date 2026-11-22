// A flow: several operations in order, sharing state through variables.
// Every request is checked automatically (request, status, contract);
// test() adds the assertions specific to the scenario.
//   lazyapi run examples/openapi.yml --flow examples/flows/userLifecycle.js
import { login, bearer } from "../tests/helpers.js";

const auth = bearer(login());

const created = lazyapi.send("createUser", {
  headers: auth,
  body: { name: "Carla Dias", email: "carla@example.com" },
});
test("create returns 201 with an id", () => {
  expect(created).toHaveStatus(201);
  expect(created.json().id).toBeGreaterThan(0);
});
vars.set("userId", created.json().id);

const fetched = lazyapi.send("getUser", { params: { id: vars.get("userId") }, headers: auth });
test("the user can be fetched", () => expect(fetched).toHaveStatus(200));

const removed = lazyapi.send("deleteUser", { params: { id: vars.get("userId") }, headers: auth });
test("the user can be deleted", () => expect(removed).toHaveStatus(204));

console.log(`lifecycle done for user ${vars.get("userId")}`);
