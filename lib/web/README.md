# web — Ring-style HTTP server library

A small HTTP/HTTPS server for jig/lisp, modelled on Clojure's
[Ring](https://github.com/ring-clojure/ring): a **request** is a
hash-map, a **response** is a hash-map, a **handler** is a function
`(fn [request] response)`, and **middleware** is a function that wraps a
handler `(fn [handler] (fn [request] response))`. Everything is data, so
handlers are ordinary functions — trivial to unit-test with `deftest`,
no socket required.

## A first server

```clojure
(def app
  (-> (web-router
        [["/ping"        {:get (fn [req] (web-json {:pong true}))}]
         ["/certs/:id"   {:get (fn [req] (web-json {:id (get (get req :path-params) :id)}))}]])
      web-wrap-json-body     ; decodes a JSON body under :json
      web-wrap-recover))     ; errors become 500 instead of dropping the connection

(web-serve {:port 8443
            :handler app
            :tls {:cert "server.crt" :key "server.key"}})
```

`web-serve` blocks until interrupted (Ctrl-C / SIGTERM) and shuts down
gracefully. Omit `:tls` for plain HTTP during development.

Test it:

```sh
curl -sk https://localhost:8443/ping        # {"pong":true}
curl -sk https://localhost:8443/certs/42     # {"id":"42"}
```

## The request map

| key | value |
|---|---|
| `:method` | keyword, lower-case (`:get`, `:post`, …) |
| `:uri` | path, e.g. `"/certs/42"` |
| `:path-params` | hash-map of names captured by the router (`{:id "42"}`) |
| `:query` | hash-map of query params (string keys) |
| `:headers` | hash-map of headers (lower-case string keys) |
| `:body` | request body as a string |
| `:scheme` | `:http` or `:https` |
| `:remote-addr` | client address |
| `:mtls` | client-certificate identity, when mTLS verified it (below) |
| `:json` | decoded JSON body, added by `web-wrap-json-body` |
| `:identity` | added by an auth middleware |

## The response map

`{:status 200 :headers {"content-type" "…"} :body "…"}`. Build it with
the helpers: `web-response`, `web-text`, `web-json`, `web-not-found`,
`web-bad-request`, `web-unauthorized`, `web-redirect`. `web-json`
encodes with clean keys (`:id` → `"id"`), unlike core `json-encode`.

## TLS and mTLS

```clojure
:tls {:cert "server.crt"
      :key  "server.key"
      :client-ca "ca.crt"          ; enables mTLS: verify client certs against this CA
      :client-auth :require-and-verify}
```

`:client-auth` is one of `:none`, `:request`, `:require`,
`:verify-if-given`, `:require-and-verify` (the default once a
`:client-ca` is given). When a client certificate is verified, the
request carries `:mtls {:subject-cn … :subject … :issuer-cn … :serial
"0x…" :sans-dns […] :verified true}`; `web-wrap-identity` promotes it to
`:identity {:kind :mtls :subject …}`.

## JWT (Keycloak)

```clojure
(def secured
  (-> protected-app
      (web-wrap-jwt {:jwks-uri "https://kc.example/realms/app/protocol/openid-connect/certs"
                     :issuer   "https://kc.example/realms/app"
                     :audience "my-api"})))
```

`web-wrap-jwt` reads the `Authorization: Bearer …` header, verifies the
token against the JWKS (RS256/384/512 and ES256/384/512 — what Keycloak
issues), checks issuer/audience/expiry, and sets `:identity {:kind :jwt
:claims … :subject …}`; a missing or invalid token gets a 401. The JWKS
is fetched and cached (auto-refreshing). `web-verify-jwt` is the
underlying primitive if you need the claims directly.

## Middleware

Middleware is `handler → handler`; compose with `->`. Built-in:
`web-wrap-recover`, `web-wrap-json-body`, `web-wrap-identity`,
`web-wrap-jwt`. Write your own the same way:

```clojure
(defn wrap-require-role [role handler]
  (fn [req]
    (if (contains? (get-in req [:identity :claims :realm_access :roles]) role)
      (handler req)
      (web-unauthorized "forbidden"))))
```

For an access log, combine it with `lib/log`:

```clojure
(defn wrap-log [handler]
  (fn [req]
    (let [start (time-ms)
          resp  (handler req)]
      (log-info "http request"
        :method (get req :method)
        :uri (get req :uri)
        :status (get resp :status)
        :ms (- (time-ms) start))
      resp)))
```

## Not yet

Streaming, Server-Sent Events, WebSockets and multipart uploads are out
of scope for now (the body is read as a string); an HTTP client is
planned separately.
