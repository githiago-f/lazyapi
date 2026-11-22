test("service is up", () => {
  expect(response).toHaveStatus(200);
  expect(response.json().status).toBe("ok");
});

test("answers fast", () => expect(response.time).toBeLessThan(2000));
