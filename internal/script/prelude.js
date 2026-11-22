// LazyAPI script prelude. Runs before every user script and defines the
// public globals on top of the native bridge `__lazyapi` provided by Go.
(function (native) {
  "use strict";
  var global = this || (0, eval)("this");

  function fmt(v) {
    if (typeof v === "string") return v;
    if (v === undefined) return "undefined";
    if (typeof v === "function") return "[Function" + (v.name ? " " + v.name : "") + "]";
    if (v instanceof Error) return v.name + ": " + v.message;
    try {
      var s = JSON.stringify(v);
      return s === undefined ? String(v) : s;
    } catch (e) {
      return String(v);
    }
  }

  // ---------------------------------------------------------------- console
  function logger(level) {
    return function () {
      var parts = [];
      for (var i = 0; i < arguments.length; i++) parts.push(fmt(arguments[i]));
      native.log(level, parts.join(" "));
    };
  }
  global.console = {
    log: logger("log"),
    info: logger("info"),
    warn: logger("warn"),
    error: logger("error"),
    debug: logger("debug"),
  };

  // --------------------------------------------------------------- response
  function LazyResponse(raw) {
    this.status = raw.status;
    this.statusText = raw.statusText;
    this.headers = raw.headers; // lower-cased names → first value
    this.body = raw.body;
    this.time = raw.time; // milliseconds
    Object.defineProperty(this, "_id", { value: raw.id, enumerable: false });
  }
  LazyResponse.prototype.header = function (name) {
    var v = this.headers[String(name).toLowerCase()];
    return v === undefined ? null : v;
  };
  LazyResponse.prototype.json = function () {
    if (this._json === undefined) {
      try {
        Object.defineProperty(this, "_json", { value: JSON.parse(this.body), enumerable: false });
      } catch (e) {
        throw new Error("response.json(): body is not valid JSON: " + e.message);
      }
    }
    return this._json;
  };
  LazyResponse.prototype.text = function () {
    return this.body;
  };

  // ---------------------------------------------------------------- request
  var rawReq = native.request;
  global.request = {
    method: rawReq.method,
    url: rawReq.url,
    path: rawReq.path,
    headers: rawReq.headers,
    query: rawReq.query,
    params: rawReq.params,
    body: rawReq.body,
    json: function () {
      return JSON.parse(rawReq.body);
    },
  };

  global.response = native.response ? new LazyResponse(native.response) : null;

  // ------------------------------------------------------------- env / vars
  function store(get, set, has, all) {
    return { get: get, set: function (k, v) { set(String(k), fmt(v)); }, has: has, all: all };
  }
  global.env = store(native.envGet, native.envSet, native.envHas, native.envAll);
  global.vars = store(native.varGet, native.varSet, native.varHas, native.varAll);

  // ---------------------------------------------------------------- lazyapi
  global.lazyapi = {
    send: function (target, opts) {
      return new LazyResponse(native.send(String(target), opts || {}));
    },
  };

  // ------------------------------------------------------------------- test
  function errMessage(e) {
    if (e && typeof e === "object" && "message" in e) return String(e.message);
    return fmt(e);
  }

  global.test = function (name, fn) {
    name = String(name);
    if (typeof fn !== "function") {
      native.record(name, false, "test() needs a function as second argument");
      return;
    }
    var out;
    try {
      out = fn();
    } catch (e) {
      native.record(name, false, errMessage(e));
      return;
    }
    if (out && typeof out.then === "function") {
      out.then(
        function () { native.record(name, true, ""); },
        function (e) { native.record(name, false, errMessage(e)); }
      );
      return;
    }
    native.record(name, true, "");
  };

  // ----------------------------------------------------------------- expect
  function deepEqual(a, b) {
    if (a === b) return true;
    if (typeof a !== "object" || typeof b !== "object" || a === null || b === null) {
      return a !== a && b !== b; // NaN
    }
    if (Array.isArray(a) !== Array.isArray(b)) return false;
    var ka = Object.keys(a), kb = Object.keys(b);
    if (ka.length !== kb.length) return false;
    for (var i = 0; i < ka.length; i++) {
      if (!Object.prototype.hasOwnProperty.call(b, ka[i]) || !deepEqual(a[ka[i]], b[ka[i]])) return false;
    }
    return true;
  }

  function getPath(obj, path) {
    var parts = Array.isArray(path) ? path : String(path).split(".");
    var cur = obj;
    for (var i = 0; i < parts.length; i++) {
      if (cur === null || cur === undefined || !Object.prototype.hasOwnProperty.call(Object(cur), parts[i])) {
        return { found: false };
      }
      cur = cur[parts[i]];
    }
    return { found: true, value: cur };
  }

  function Expectation(actual, negated) {
    this.actual = actual;
    this.negated = !!negated;
    if (!negated) this.not = new Expectation(actual, true);
  }

  Expectation.prototype._assert = function (pass, message, negMessage) {
    if (this.negated ? pass : !pass) {
      throw new Error(this.negated ? negMessage : message);
    }
  };

  var matchers = {
    toBe: function (expected) {
      this._assert(this.actual === expected,
        "expected " + fmt(this.actual) + " to be " + fmt(expected),
        "expected " + fmt(this.actual) + " not to be " + fmt(expected));
    },
    toEqual: function (expected) {
      this._assert(deepEqual(this.actual, expected),
        "expected " + fmt(this.actual) + " to equal " + fmt(expected),
        "expected " + fmt(this.actual) + " not to equal " + fmt(expected));
    },
    toBeTruthy: function () {
      this._assert(!!this.actual, "expected " + fmt(this.actual) + " to be truthy",
        "expected " + fmt(this.actual) + " to be falsy");
    },
    toBeFalsy: function () {
      this._assert(!this.actual, "expected " + fmt(this.actual) + " to be falsy",
        "expected " + fmt(this.actual) + " to be truthy");
    },
    toBeDefined: function () {
      this._assert(this.actual !== undefined, "expected value to be defined", "expected value to be undefined");
    },
    toBeUndefined: function () {
      this._assert(this.actual === undefined, "expected " + fmt(this.actual) + " to be undefined",
        "expected value to be defined");
    },
    toBeNull: function () {
      this._assert(this.actual === null, "expected " + fmt(this.actual) + " to be null", "expected value not to be null");
    },
    toBeGreaterThan: function (n) {
      this._assert(this.actual > n, "expected " + fmt(this.actual) + " to be > " + fmt(n),
        "expected " + fmt(this.actual) + " not to be > " + fmt(n));
    },
    toBeGreaterThanOrEqual: function (n) {
      this._assert(this.actual >= n, "expected " + fmt(this.actual) + " to be >= " + fmt(n),
        "expected " + fmt(this.actual) + " not to be >= " + fmt(n));
    },
    toBeLessThan: function (n) {
      this._assert(this.actual < n, "expected " + fmt(this.actual) + " to be < " + fmt(n),
        "expected " + fmt(this.actual) + " not to be < " + fmt(n));
    },
    toBeLessThanOrEqual: function (n) {
      this._assert(this.actual <= n, "expected " + fmt(this.actual) + " to be <= " + fmt(n),
        "expected " + fmt(this.actual) + " not to be <= " + fmt(n));
    },
    toContain: function (item) {
      var a = this.actual;
      var pass = typeof a === "string" ? a.indexOf(String(item)) >= 0
        : Array.isArray(a) ? a.some(function (x) { return deepEqual(x, item); }) : false;
      this._assert(pass, "expected " + fmt(a) + " to contain " + fmt(item),
        "expected " + fmt(a) + " not to contain " + fmt(item));
    },
    toMatch: function (re) {
      var pass = (re instanceof RegExp ? re : new RegExp(String(re))).test(String(this.actual));
      this._assert(pass, "expected " + fmt(this.actual) + " to match " + String(re),
        "expected " + fmt(this.actual) + " not to match " + String(re));
    },
    toHaveLength: function (n) {
      var len = this.actual == null ? undefined : this.actual.length;
      this._assert(len === n, "expected length " + fmt(len) + " to be " + fmt(n),
        "expected length not to be " + fmt(n));
    },
    toHaveProperty: function (path, value) {
      var r = getPath(this.actual, path);
      var pass = r.found && (arguments.length < 2 || deepEqual(r.value, value));
      var want = arguments.length < 2 ? "" : " = " + fmt(value);
      this._assert(pass,
        "expected object to have property " + fmt(path) + want + (r.found ? " (got " + fmt(r.value) + ")" : ""),
        "expected object not to have property " + fmt(path) + want);
    },
    toHaveStatus: function (code) {
      var s = this.actual && this.actual.status;
      this._assert(s === code, "expected status " + fmt(code) + ", got " + fmt(s),
        "expected status not to be " + fmt(code));
    },
    toHaveHeader: function (name, value) {
      var v = this.actual && typeof this.actual.header === "function" ? this.actual.header(name) : null;
      var pass = v !== null && (arguments.length < 2 ||
        (value instanceof RegExp ? value.test(v) : v === String(value)));
      var want = arguments.length < 2 ? "" : " = " + String(value);
      this._assert(pass, "expected header " + name + want + (v === null ? " (missing)" : " (got " + fmt(v) + ")"),
        "expected header " + name + want + " to be absent");
    },
    toMatchSchema: function (schema) {
      var err;
      if (schema === undefined) {
        if (!(this.actual instanceof LazyResponse)) {
          throw new Error("toMatchSchema() without a schema only works on a response");
        }
        err = native.validateContract(this.actual._id);
      } else {
        err = native.validateSchema(JSON.stringify(this.actual), JSON.stringify(schema));
      }
      this._assert(!err, "schema validation failed: " + err, "expected value not to match schema");
    },
  };
  Object.keys(matchers).forEach(function (k) { Expectation.prototype[k] = matchers[k]; });

  global.expect = function (actual) { return new Expectation(actual, false); };
}).call(this, __lazyapi);
