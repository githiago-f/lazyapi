const users = response.json();

test("returns a list of users", () => {
  expect(Array.isArray(users)).toBe(true);
  expect(users.length).toBeGreaterThan(0);
});

test("every user has an email and a role", () => {
  for (const u of users) {
    expect(u.email).toMatch(/@/);
    expect(["admin", "member"]).toContain(u.role);
  }
});

// Later operations in the same run can use {{var.firstUserId}}.
vars.set("firstUserId", users[0].id);
console.log(`${users.length} users`);
