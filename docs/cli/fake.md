# `lazyapi fake`

Lists the built-in dynamic variables (`{{$name}}`), or generates one value.

```bash
lazyapi fake                     # table: template, faker method, description, example
lazyapi fake --json              # the same as JSON
lazyapi fake randomEmail         # one value (the leading $ is optional)
```

Use them in URLs, params, headers and bodies (`{{$randomEmail}}`), or in scripts (`faker.email()`). Each occurrence gets a new value when the request is sent. A test script can add its own or replace a built-in by exporting a function of the same name: see [Dynamic values and helpers](/scripting#dynamic-values-and-helpers).
