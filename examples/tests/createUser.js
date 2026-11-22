const user = response.json();

test("creates the user", () => {
  expect(response).toHaveStatus(201);
  expect(user.email).toBe(request.json().email);
});

test("new users are members", () => expect(user.role).toBe("member"));

// after(res) runs once the tests are done, even if they failed: remove what
// this test created so the run can be repeated against a real API.
export function after(res) {
  if (res.status === 201) {
    lazyapi.send("deleteUser", { params: { id: String(res.json().id) } });
  }
}
