const user = response.json();

test("creates the user", () => {
  expect(response).toHaveStatus(201);
  expect(user.email).toBe(request.json().email);
});

test("new users are members", () => expect(user.role).toBe("member"));
