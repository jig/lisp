;; $MODULE header-web

;; Ring-style response helpers and middleware for the web library.
;; A response is {:status :headers :body}; a handler is (fn [req] resp);
;; middleware is (fn [handler] (fn [req] resp)). Compose middleware with
;; -> , e.g. (-> app http/wrap-json-body http/wrap-log http/wrap-recover).
(do
    (defn web/response
        "Builds a response map with the given status and body, plus optional header pairs."
        [status body & headers]
        {:status status
         :headers (apply hash-map headers)
         :body body})

    (defn web/text
        "A text/plain response (status 200 unless given)."
        [body & status]
        (web/response (if (empty? status) 200 (first status)) body "content-type" "text/plain; charset=utf-8"))

    (defn web/json
        "A JSON response: encodes body and sets content-type. (web/json data) is 200; (web/json status data) sets the status."
        [status-or-body & maybe-body]
        (let [status (if (empty? maybe-body) 200 status-or-body)
              body   (if (empty? maybe-body) status-or-body (first maybe-body))]
            (web/response status (web/encode-json body) "content-type" "application/json")))

    (defn web/not-found
        "A 404 JSON response."
        [& msg]
        (web/json 404 {:error (if (empty? msg) "not found" (first msg))}))

    (defn web/bad-request
        "A 400 JSON response."
        [& msg]
        (web/json 400 {:error (if (empty? msg) "bad request" (first msg))}))

    (defn web/unauthorized
        "A 401 JSON response."
        [& msg]
        (web/json 401 {:error (if (empty? msg) "unauthorized" (first msg))}))

    (defn web/redirect
        "A redirect response (status 302 unless given as the second arg)."
        [location & status]
        (web/response (if (empty? status) 302 (first status)) "" "location" location))

    ;; --- middleware ---

    (defn web/wrap-recover
        "Middleware: turns any error escaping the handler into a 500 JSON response instead of dropping the connection."
        [handler]
        (fn [req]
            (try
                (handler req)
                (catch e (web/json 500 {:error (str e)})))))

    (defn web/wrap-json-body
        "Middleware: when the request body is a non-empty JSON object, decodes it under :json (nil on parse error)."
        [handler]
        (fn [req]
            (let [body (get req :body)]
                (if (and (string? body) (not (= "" body)))
                    (handler (assoc req :json (try (json-decode {} body) (catch e nil))))
                    (handler req)))))

    (defn web/wrap-log
        "Middleware: logs one structured JSON line per request with method, uri, status and elapsed ms."
        [handler]
        (fn [req]
            (let [start (time-ms)
                  resp  (handler req)]
                (web/log :info "http request"
                    "method" (get req :method)
                    "uri" (get req :uri)
                    "status" (get resp :status)
                    "ms" (- (time-ms) start))
                resp)))

    (defn web/wrap-identity
        "Middleware: promotes a verified mTLS client certificate to :identity {:kind :mtls :subject cn}."
        [handler]
        (fn [req]
            (let [mtls (get req :mtls)]
                (if (and mtls (get mtls :verified))
                    (handler (assoc req :identity {:kind :mtls :subject (get mtls :subject-cn) :mtls mtls}))
                    (handler req)))))

    (defn web/-bearer-token
        "Extracts the bearer token from a request's Authorization header, or nil."
        [req]
        (let [auth (get (get req :headers) "authorization")]
            (if (and (string? auth) (starts-with? auth "Bearer "))
                (subs auth 7)
                nil)))

    (defn web/wrap-jwt
        "Middleware: verifies a Bearer JWT against config (see web/verify-jwt) and sets :identity {:kind :jwt :claims …}; responds 401 when missing or invalid. config is the JWKS/issuer map."
        [config handler]
        (fn [req]
            (let [token (web/-bearer-token req)]
                (if (nil? token)
                    (web/unauthorized "missing bearer token")
                    (let [claims (try (web/verify-jwt token config) (catch e :invalid))]
                        (if (= claims :invalid)
                            (web/unauthorized "invalid token")
                            (handler (assoc req :identity {:kind :jwt :claims claims :subject (get claims :sub)}))))))))
)
