// Postman compatibility layer: lets imported Postman scripts run unchanged.
// pm.test/pm.expect map to test/expect (with a Chai-style BDD subset),
// pm.response wraps `response`, and every variable scope maps to `env`.
(function () {
  "use strict";
  var global = this;

  function fmt(v) {
    try { return typeof v === "string" ? JSON.stringify(v) : JSON.stringify(v) || String(v); } catch (e) { return String(v); }
  }
  function deepEqual(a, b) {
    if (a === b) return true;
    if (typeof a !== "object" || typeof b !== "object" || a === null || b === null) return a !== a && b !== b;
    if (Array.isArray(a) !== Array.isArray(b)) return false;
    var ka = Object.keys(a), kb = Object.keys(b);
    if (ka.length !== kb.length) return false;
    for (var i = 0; i < ka.length; i++) if (!deepEqual(a[ka[i]], b[ka[i]])) return false;
    return true;
  }
  function typeOf(v) {
    if (v === null) return "null";
    if (Array.isArray(v)) return "array";
    return typeof v;
  }

  // ------------------------------------------------------------- Chai subset
  function Assertion(obj) {
    this._obj = obj;
    this._not = false;
    this._deep = false;
  }
  var A = Assertion.prototype;
  ["to", "be", "been", "is", "that", "which", "and", "has", "have", "with", "at", "of", "same", "does", "also", "still"].forEach(function (w) {
    Object.defineProperty(A, w, { get: function () { return this; } });
  });
  Object.defineProperty(A, "not", { get: function () { this._not = !this._not; return this; } });
  Object.defineProperty(A, "deep", { get: function () { this._deep = true; return this; } });
  A._assert = function (ok, msg, negMsg) {
    if (this._not ? ok : !ok) throw new Error(this._not ? negMsg : msg);
    return this;
  };
  function getter(name, check, msg) {
    Object.defineProperty(A, name, {
      get: function () { return this._assert(check(this._obj), "expected " + fmt(this._obj) + " to be " + msg, "expected " + fmt(this._obj) + " not to be " + msg); },
    });
  }
  getter("ok", function (v) { return !!v; }, "truthy");
  getter("true", function (v) { return v === true; }, "true");
  getter("false", function (v) { return v === false; }, "false");
  getter("null", function (v) { return v === null; }, "null");
  getter("undefined", function (v) { return v === undefined; }, "undefined");
  getter("NaN", function (v) { return v !== v; }, "NaN");
  getter("exist", function (v) { return v !== null && v !== undefined; }, "defined");
  getter("empty", function (v) {
    if (v == null) return true;
    if (typeof v === "string" || Array.isArray(v)) return v.length === 0;
    return typeof v === "object" && Object.keys(v).length === 0;
  }, "empty");

  A.equal = A.equals = A.eq = function (v) {
    var ok = this._deep ? deepEqual(this._obj, v) : this._obj === v;
    return this._assert(ok, "expected " + fmt(this._obj) + " to equal " + fmt(v), "expected " + fmt(this._obj) + " not to equal " + fmt(v));
  };
  A.eql = A.eqls = function (v) {
    return this._assert(deepEqual(this._obj, v), "expected " + fmt(this._obj) + " to deeply equal " + fmt(v), "expected " + fmt(this._obj) + " not to deeply equal " + fmt(v));
  };
  A.a = A.an = function (type) {
    var t = typeOf(this._obj);
    return this._assert(t === String(type).toLowerCase(), "expected " + fmt(this._obj) + " to be a " + type, "expected " + fmt(this._obj) + " not to be a " + type);
  };
  A.above = A.gt = A.greaterThan = function (n) { return this._assert(this._obj > n, "expected " + fmt(this._obj) + " to be above " + n, "expected " + fmt(this._obj) + " to be at most " + n); };
  A.below = A.lt = A.lessThan = function (n) { return this._assert(this._obj < n, "expected " + fmt(this._obj) + " to be below " + n, "expected " + fmt(this._obj) + " to be at least " + n); };
  A.least = A.gte = function (n) { return this._assert(this._obj >= n, "expected " + fmt(this._obj) + " to be at least " + n, "expected " + fmt(this._obj) + " to be below " + n); };
  A.most = A.lte = function (n) { return this._assert(this._obj <= n, "expected " + fmt(this._obj) + " to be at most " + n, "expected " + fmt(this._obj) + " to be above " + n); };
  A.within = function (lo, hi) { return this._assert(this._obj >= lo && this._obj <= hi, "expected " + fmt(this._obj) + " to be within " + lo + ".." + hi, "expected " + fmt(this._obj) + " not to be within " + lo + ".." + hi); };
  A.include = A.includes = A.contain = A.contains = function (v) {
    var o = this._obj, ok;
    if (typeof o === "string") ok = o.indexOf(v) >= 0;
    else if (Array.isArray(o)) ok = o.some(function (x) { return deepEqual(x, v); });
    else if (o && typeof o === "object" && v && typeof v === "object") ok = Object.keys(v).every(function (k) { return deepEqual(o[k], v[k]); });
    else ok = false;
    return this._assert(ok, "expected " + fmt(o) + " to include " + fmt(v), "expected " + fmt(o) + " not to include " + fmt(v));
  };
  A.property = function (name, value) {
    var o = this._obj;
    var has = o != null && Object.prototype.hasOwnProperty.call(Object(o), name);
    var ok = has && (arguments.length < 2 || deepEqual(o[name], value));
    this._assert(ok, "expected " + fmt(o) + " to have property " + fmt(name) + (arguments.length > 1 ? " of " + fmt(value) : ""),
      "expected " + fmt(o) + " not to have property " + fmt(name));
    if (has && !this._not) this._obj = o[name];
    return this;
  };
  A.lengthOf = A.length = function (n) {
    var len = this._obj == null ? undefined : this._obj.length;
    return this._assert(len === n, "expected length " + len + " to be " + n, "expected length not to be " + n);
  };
  A.match = A.matches = function (re) { return this._assert(re.test(String(this._obj)), "expected " + fmt(this._obj) + " to match " + re, "expected " + fmt(this._obj) + " not to match " + re); };
  A.oneOf = function (list) { return this._assert(list.some(function (x) { return deepEqual(x, this._obj); }, this), "expected " + fmt(this._obj) + " to be one of " + fmt(list), "expected " + fmt(this._obj) + " not to be one of " + fmt(list)); };
  A.keys = A.key = function () {
    var want = Array.isArray(arguments[0]) ? arguments[0] : Array.prototype.slice.call(arguments);
    var o = this._obj || {};
    return this._assert(want.every(function (k) { return Object.prototype.hasOwnProperty.call(o, k); }), "expected " + fmt(o) + " to have keys " + fmt(want), "expected " + fmt(o) + " not to have keys " + fmt(want));
  };
  A.status = function (code) {
    var r = this._obj;
    var actual = r && (r.code !== undefined ? r.code : r.status);
    var ok = typeof code === "number" ? actual === code : r && String(r.status).toLowerCase() === String(code).toLowerCase();
    return this._assert(ok, "expected response to have status " + code + " but got " + actual, "expected response not to have status " + code);
  };
  A.header = function (name, value) {
    var r = this._obj;
    var v = r && r.headers && r.headers.get(name);
    var ok = v !== undefined && v !== null && (arguments.length < 2 || v === value);
    return this._assert(ok, "expected response to have header " + name + (arguments.length > 1 ? ": " + value : ""), "expected response not to have header " + name);
  };
  A.jsonBody = function (path, value) {
    var body;
    try { body = this._obj.json(); } catch (e) { return this._assert(false, "expected response to have a JSON body", ""); }
    if (path === undefined) return this._assert(true, "", "expected response not to have a JSON body");
    var cur = body, parts = String(path).split(".");
    for (var i = 0; i < parts.length; i++) { if (cur == null) break; cur = cur[parts[i]]; }
    return this._assert(cur !== undefined && (arguments.length < 2 || deepEqual(cur, value)), "expected JSON body to have " + path, "expected JSON body not to have " + path);
  };
  Object.defineProperty(A, "json", { get: function () { return this.jsonBody(); } });
  Object.defineProperty(A, "success", { get: function () { var c = this._obj.code; return this._assert(c >= 200 && c < 300, "expected a 2xx response, got " + c, "expected a non-2xx response"); } });

  // --------------------------------------------------------------- pm.*
  function scope() {
    return {
      get: function (k) { return env.get(k); },
      set: function (k, v) { env.set(k, v); },
      unset: function (k) { env.set(k, ""); },
      has: function (k) { return env.has(k); },
      toObject: function () { return env.all(); },
      replaceIn: function (s) { return String(s).replace(/\{\{\s*([\w.\-]+)\s*\}\}/g, function (m, k) { var v = env.get(k); return v === undefined ? m : v; }); },
    };
  }
  function headerList(obj) {
    return {
      get: function (n) { var k = Object.keys(obj).filter(function (x) { return x.toLowerCase() === String(n).toLowerCase(); })[0]; return k === undefined ? undefined : obj[k]; },
      has: function (n) { return this.get(n) !== undefined; },
      add: function (h) { obj[h.key] = String(h.value); },
      upsert: function (h) { this.remove(h.key); obj[h.key] = String(h.value); },
      remove: function (n) { Object.keys(obj).forEach(function (k) { if (k.toLowerCase() === String(n).toLowerCase()) delete obj[k]; }); },
      toObject: function () { return obj; },
    };
  }

  function pmHeaders(h) {
    var out = {};
    if (!h) return out;
    if (Array.isArray(h)) h.forEach(function (x) { if (x && x.key && !x.disabled) out[x.key] = String(x.value); });
    else if (typeof h === "string") h.split("\n").forEach(function (line) { var i = line.indexOf(":"); if (i > 0) out[line.slice(0, i).trim()] = line.slice(i + 1).trim(); });
    else Object.keys(h).forEach(function (k) { out[k] = String(h[k]); });
    return out;
  }
  // pmBody converts a Postman body ({mode: raw|urlencoded|formdata}) to a
  // lazyapi.send body.
  function pmBody(b, headers) {
    if (b === undefined || b === null) return undefined;
    if (typeof b === "string") return b;
    var fields = function (list) { var o = {}; (list || []).forEach(function (x) { if (x && x.key && !x.disabled) o[x.key] = x.value; }); return o; };
    switch (b.mode) {
      case "raw": return b.raw;
      case "urlencoded":
        if (!Object.keys(headers).some(function (k) { return k.toLowerCase() === "content-type"; })) headers["Content-Type"] = "application/x-www-form-urlencoded";
        var o = fields(b.urlencoded);
        return Object.keys(o).map(function (k) { return encodeURIComponent(k) + "=" + encodeURIComponent(o[k]); }).join("&");
      case "formdata": throw new Error("pm.sendRequest: formdata bodies are not supported (file parts are only sent with --file)");
      default: return b.raw !== undefined ? b.raw : b;
    }
  }
  function wrapResponse(res) {
    var wrapped = {
      code: res.status,
      status: String(res.statusText).replace(/^\d+\s*/, ""),
      responseTime: res.time,
      responseSize: res.body.length,
      headers: headerList(res.headers),
      json: function () { return res.json(); },
      text: function () { return res.body; },
    };
    Object.defineProperty(wrapped, "to", { get: function () { return new Assertion(wrapped); } });
    return wrapped;
  }

  var current = null; // the request object while pm.__before runs
  var pm = {
    test: function (name, fn) { return global.test(name, fn); },
    expect: function (v) { return new Assertion(v); },
    environment: scope(), variables: scope(), collectionVariables: scope(), globals: scope(), iterationData: scope(),
    info: { eventName: "test", iteration: 0, iterationCount: 1, requestName: "" },
    // pm.sendRequest(url | {url, method, header, body}, callback): the URL is
    // mapped to the spec operation it belongs to and sent with lazyapi.send,
    // so it gets the same checks. The callback runs before sendRequest
    // returns (scripts are synchronous); the response is also returned.
    sendRequest: function (req, callback) {
      var r = typeof req === "string" ? { url: req } : (req || {});
      var url = typeof r.url === "string" ? r.url : (r.url && (r.url.raw || String(r.url))) || "";
      var opts = { headers: pmHeaders(r.header), query: {} };
      var body = pmBody(r.body, opts.headers);
      if (body !== undefined) opts.body = body;
      var err = null, res = null;
      try {
        res = wrapResponse(lazyapi.send(String(r.method || "GET").toUpperCase() + " " + scope().replaceIn(url), opts));
      } catch (e) {
        err = e;
      }
      if (typeof callback === "function") callback(err, res);
      else if (err) throw err;
      return res;
    },
    __before: function (req, fn) {
      current = req;
      pm.info.eventName = "prerequest";
      try { fn(); } finally { current = null; pm.info.eventName = "test"; }
    },
  };
  Object.defineProperty(pm, "request", {
    get: function () {
      var r = current || global.request;
      return {
        method: r.method,
        url: { toString: function () { return r.url; }, getPath: function () { return r.path; }, query: headerList(r.query) },
        headers: headerList(r.headers),
        body: { get raw() { return typeof r.body === "string" ? r.body : JSON.stringify(r.body); }, set raw(v) { r.body = v; } },
      };
    },
  });
  Object.defineProperty(pm, "response", {
    get: function () {
      var res = global.response;
      if (!res) throw new Error("pm.response is not available before the response arrives");
      return wrapResponse(res);
    },
  });
  global.pm = pm;
  global.postman = { setEnvironmentVariable: function (k, v) { env.set(k, v); }, getEnvironmentVariable: function (k) { return env.get(k); } };
}).call(this);
