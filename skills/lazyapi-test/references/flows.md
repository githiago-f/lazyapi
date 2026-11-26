# Flows: scenarios across operations

A flow is `.lazyapi/flows/<name>.js` next to the spec. It has no implicit request: it calls operations in order with `lazyapi.send()` and asserts with `test()`. **Every step is checked automatically** — the request matches the spec, the status is declared, the response matches the schema — and named after the step (`#2 POST /users · status 201 is declared`). A flow with no requests and no assertions fails.

```js
// .lazyapi/flows/user-lifecycle.js
const login = lazyapi.send("login", { body: { user: env.get("USER"), password: env.get("PASSWORD") } });
test("logged in", () => expect(login).toHaveStatus(200));
const auth = { Authorization: "Bearer " + login.json().token };

const created = lazyapi.send("createUser", { headers: auth, body: { name: "Ana", email: faker.email() } });
test("created", () => expect(created).toHaveStatus(201));
const id = created.json().id;

const read = lazyapi.send("getUser", { params: { id }, headers: auth });
test("readable", () => expect(read.json()).toHaveProperty("name", "Ana"));

const removed = lazyapi.send("deleteUser", { params: { id }, headers: auth });
test("deleted", () => expect(removed).toHaveStatus(204));
```

- Write with MCP `write_flow {spec, name: "user-lifecycle", source}` (returns compile errors), run with `run_flow {spec, flow: "user-lifecycle"}` or `lazyapi run <spec> --flow .lazyapi/flows/user-lifecycle.js`.
- `lazyapi run <spec> --flows` runs the operations, then every flow; flows share `env` and `vars` with them.
- Flows usually write data: they run only when asked for, and against a server the user agreed to (a mock or a test environment). Delete what you create, last step first.
- Results carry `kind: "flow"` and `steps` (method, path, status, time per step).
