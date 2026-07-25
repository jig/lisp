// Package web is a Ring-style HTTP server library for jig/lisp.
//
// The contract mirrors Clojure's Ring: an HTTP request is a hash-map, a
// response is a hash-map {:status :headers :body}, a handler is a
// function (fn [request] response), and middleware is a function that
// wraps a handler (fn [handler] (fn [request] response)). The Go side
// provides the transport (http/serve with TLS and mTLS), a data-driven
// router (http/router), and JWT verification (http/verify-jwt); the
// response helpers and middleware live in header-web.lisp.
package web

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"github.com/jig/lisp/lib/call"
	"github.com/jig/lisp/printer"
	. "github.com/jig/lisp/types"
)

//go:embed header-web.lisp
var headerWeb string

// HeaderWeb returns the Lisp source of the response helpers and
// middleware (see nsweb.Load).
func HeaderWeb() string { return headerWeb }

// --- value helpers -------------------------------------------------------

func kw(s string) Keyword { return KW(s) }

// kwName returns a keyword's name (:get → "get"), a string verbatim,
// and anything else rendered via toStr.
func kwName(v MalType) string {
	switch v := v.(type) {
	case Keyword:
		return string(v)
	case string:
		return v
	default:
		return toStr(v)
	}
}

// hget reads a keyword-keyed value from a hash-map.
func hget(m MalType, key string) (MalType, bool) {
	hm, ok := m.(HashMap)
	if !ok {
		return nil, false
	}
	v, ok := hm.Items[kw(key)]
	return v, ok
}

func hgetStr(m MalType, key, def string) string {
	if v, ok := hget(m, key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func hgetInt(m MalType, key string, def int) int {
	if v, ok := hget(m, key); ok {
		if i, ok := v.(int); ok {
			return i
		}
	}
	return def
}

// toStr renders a MalType as the raw string that should go on the wire:
// strings verbatim, everything else via the Lisp printer.
func toStr(v MalType) string {
	if s, ok := v.(string); ok {
		return s
	}
	if v == nil {
		return ""
	}
	return printer.Pr_str(v, false)
}

// headerName turns a hash-map key (a plain string or a keyword) into an
// HTTP header name.
func headerName(k MalType) string {
	return http.CanonicalHeaderKey(kwName(k))
}

// --- JSON <-> Lisp (for JWT claims) --------------------------------------

// jsonToMal converts decoded JSON into Lisp data: objects become
// hash-maps keyed by keywords, arrays become vectors, integral numbers
// become ints.
func jsonToMal(v any) MalType {
	switch v := v.(type) {
	case map[string]any:
		out := make(map[MalType]MalType, len(v))
		for k, val := range v {
			out[kw(k)] = jsonToMal(val)
		}
		return HashMap{Items: out}
	case []any:
		out := make([]MalType, len(v))
		for i, e := range v {
			out[i] = jsonToMal(e)
		}
		return Vector{Val: out}
	case float64:
		if v == float64(int(v)) {
			return int(v)
		}
		return float32(v)
	default:
		return v // string, bool, nil
	}
}

// malToJSON converts Lisp data to a Go value ready for json.Marshal,
// producing clean JSON for HTTP clients: keyword map keys and keyword
// values lose their sigil (:id → "id", :active → "active"), since JSON
// has no keyword type — the same convention core's json-encode follows.
func malToJSON(v MalType) any {
	switch v := v.(type) {
	case Keyword:
		return string(v)
	case string:
		return v
	case HashMap:
		m := make(map[string]any, len(v.Items))
		for k, val := range v.Items {
			m[kwName(k)] = malToJSON(val)
		}
		return m
	case List:
		out := make([]any, len(v.Val))
		for i, e := range v.Val {
			out[i] = malToJSON(e)
		}
		return out
	case Vector:
		out := make([]any, len(v.Val))
		for i, e := range v.Val {
			out[i] = malToJSON(e)
		}
		return out
	case int, float32, float64, bool, nil:
		return v
	default:
		return printer.Pr_str(v, false)
	}
}

// webEncodeJSON encodes Lisp data as a JSON string for HTTP responses.
func webEncodeJSON(v MalType) (MalType, error) {
	b, err := json.Marshal(malToJSON(v))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// --- request map ---------------------------------------------------------

// requestMap builds the Ring request hash-map from an *http.Request.
func requestMap(r *http.Request) (HashMap, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return HashMap{}, err
	}

	headers := make(map[MalType]MalType, len(r.Header))
	for name, vals := range r.Header {
		if len(vals) > 0 {
			headers[strings.ToLower(name)] = vals[0]
		}
	}
	query := make(map[MalType]MalType)
	for name, vals := range r.URL.Query() {
		if len(vals) > 0 {
			query[name] = vals[0]
		}
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	m := map[MalType]MalType{
		kw("method"):      KW(strings.ToLower(r.Method)),
		kw("uri"):         r.URL.Path,
		kw("query"):       HashMap{Items: query},
		kw("headers"):     HashMap{Items: headers},
		kw("body"):        string(body),
		kw("remote-addr"): r.RemoteAddr,
		kw("scheme"):      KW(scheme),
		kw("protocol"):    r.Proto,
		kw("path-params"): HashMap{Items: map[MalType]MalType{}},
	}

	// mTLS: when the transport verified a client certificate, expose its
	// identity so an authorisation middleware can consume it.
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		leaf := r.TLS.PeerCertificates[0]
		sans := make([]MalType, len(leaf.DNSNames))
		for i, s := range leaf.DNSNames {
			sans[i] = s
		}
		m[kw("mtls")] = HashMap{Items: map[MalType]MalType{
			kw("subject-cn"): leaf.Subject.CommonName,
			kw("subject"):    leaf.Subject.String(),
			kw("issuer-cn"):  leaf.Issuer.CommonName,
			kw("serial"):     "0x" + strings.ToUpper(leaf.SerialNumber.Text(16)),
			kw("sans-dns"):   Vector{Val: sans},
			kw("verified"):   true,
		}}
	}
	return HashMap{Items: m}, nil
}

// writeResponse writes a Ring response hash-map to the ResponseWriter.
func writeResponse(w http.ResponseWriter, resp MalType) {
	hm, ok := resp.(HashMap)
	if !ok {
		// A bare string is a 200 text/plain body; anything else is a bug.
		if s, ok := resp.(string); ok {
			_, _ = io.WriteString(w, s)
			return
		}
		http.Error(w, "handler did not return a response map", http.StatusInternalServerError)
		return
	}
	if h, ok := hget(hm, "headers"); ok {
		if hh, ok := h.(HashMap); ok {
			for k, v := range hh.Items {
				w.Header().Set(headerName(k), toStr(v))
			}
		}
	}
	w.WriteHeader(hgetInt(hm, "status", http.StatusOK))
	if b, ok := hget(hm, "body"); ok {
		_, _ = io.WriteString(w, toStr(b))
	}
}

// --- TLS -----------------------------------------------------------------

var clientAuthTypes = map[string]tls.ClientAuthType{
	"none":                    tls.NoClientCert,
	"request":                 tls.RequestClientCert,
	"require":                 tls.RequireAnyClientCert,
	"verify-if-given":         tls.VerifyClientCertIfGiven,
	"require-and-verify":      tls.RequireAndVerifyClientCert,
	"require-and-verify-mtls": tls.RequireAndVerifyClientCert,
}

// tlsConfig builds a *tls.Config from the :tls sub-map, or nil for plain
// HTTP when :tls is absent.
func tlsConfig(config MalType) (*tls.Config, error) {
	tlsMap, ok := hget(config, "tls")
	if !ok {
		return nil, nil
	}
	certFile := hgetStr(tlsMap, "cert", "")
	keyFile := hgetStr(tlsMap, "key", "")
	if certFile == "" || keyFile == "" {
		return nil, errors.New("web-serve: :tls requires :cert and :key")
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("web-serve: loading server certificate: %w", err)
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	if caFile := hgetStr(tlsMap, "client-ca", ""); caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("web-serve: reading client CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errors.New("web-serve: no certificates found in :client-ca")
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert // default when a CA is given
	}
	if v, ok := hget(tlsMap, "client-auth"); ok {
		name := kwName(v)
		at, ok := clientAuthTypes[name]
		if !ok {
			return nil, fmt.Errorf("web-serve: unknown :client-auth %q", name)
		}
		cfg.ClientAuth = at
	}
	return cfg, nil
}

// ringHandler adapts a Lisp Ring handler (a fn taking the request
// hash-map and returning a response hash-map) to a net/http handler. It
// is the single request→lisp→response bridge, shared by webServe and the
// tests; a panic or a returned error becomes a 500.
func ringHandler(handler MalType) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				http.Error(w, fmt.Sprintf("panic: %v", rec), http.StatusInternalServerError)
			}
		}()
		req, err := requestMap(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, err := Apply(r.Context(), handler, []MalType{req})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeResponse(w, resp)
	})
}

// --- serve ---------------------------------------------------------------

// webServe starts an HTTP(S) server and blocks until the context is
// cancelled or an interrupt signal arrives, then shuts down gracefully.
func webServe(ctx context.Context, config MalType) (MalType, error) {
	handler, ok := hget(config, "handler")
	if !ok {
		return nil, errors.New("web-serve: config needs a :handler function")
	}
	addr := hgetStr(config, "addr", "")
	if addr == "" {
		port := hgetInt(config, "port", 8080)
		addr = fmt.Sprintf(":%d", port)
	}
	cfg, err := tlsConfig(config)
	if err != nil {
		return nil, err
	}

	srv := &http.Server{
		Addr:      addr,
		TLSConfig: cfg,
		Handler:   ringHandler(handler),
	}

	// Graceful shutdown on context cancellation or SIGINT/SIGTERM.
	shutdownCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdownCtx.Done()
		toCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(toCtx)
	}()

	if cfg != nil {
		err = srv.ListenAndServeTLS("", "")
	} else {
		err = srv.ListenAndServe()
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil, nil
	}
	return nil, err
}

// --- router --------------------------------------------------------------

type route struct {
	segments []string           // path split on "/", ":name" captures a param
	methods  map[string]MalType // lower-case method → handler
}

// compileRoutes parses the route data: a vector of [path method-map]
// pairs, where path is "/a/:id/b" and method-map is {:get fn :post fn}.
func compileRoutes(routes MalType) ([]route, error) {
	slc, err := GetSlice(routes)
	if err != nil {
		return nil, errors.New("web-router: routes must be a vector of [path methods] pairs")
	}
	out := make([]route, 0, len(slc))
	for _, entry := range slc {
		pair, err := GetSlice(entry)
		if err != nil || len(pair) != 2 {
			return nil, errors.New("web-router: each route must be a [path {:method handler}] pair")
		}
		path, ok := pair[0].(string)
		if !ok {
			return nil, errors.New("web-router: route path must be a string")
		}
		mm, ok := pair[1].(HashMap)
		if !ok {
			return nil, errors.New("web-router: route methods must be a hash-map")
		}
		methods := make(map[string]MalType, len(mm.Items))
		for k, v := range mm.Items {
			methods[strings.ToLower(kwName(k))] = v
		}
		out = append(out, route{segments: splitPath(path), methods: methods})
	}
	return out, nil
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}

// matchRoute matches a request path against a compiled route, returning
// the captured path params on success.
func matchRoute(r route, path []string) (map[MalType]MalType, bool) {
	if len(r.segments) != len(path) {
		return nil, false
	}
	params := map[MalType]MalType{}
	for i, seg := range r.segments {
		if strings.HasPrefix(seg, ":") {
			params[kw(seg[1:])] = path[i]
			continue
		}
		if seg != path[i] {
			return nil, false
		}
	}
	return params, true
}

// notFound and methodNotAllowed are the router's built-in fallbacks.
func statusResponse(status int, msg string) HashMap {
	return HashMap{Items: map[MalType]MalType{
		kw("status"):  status,
		kw("headers"): HashMap{Items: map[MalType]MalType{"content-type": "text/plain; charset=utf-8"}},
		kw("body"):    msg,
	}}
}

// webRouter compiles the routes and returns a handler Func that
// dispatches on method and path, filling :path-params.
func webRouter(routes MalType) (MalType, error) {
	compiled, err := compileRoutes(routes)
	if err != nil {
		return nil, err
	}
	fn := func(ctx context.Context, args []MalType) (MalType, error) {
		if len(args) != 1 {
			return nil, errors.New("router handler takes one request argument")
		}
		req := args[0]
		path := splitPath(hgetStr(req, "uri", "/"))
		method := kwName(mustGet(req, "method"))
		pathMatched := false
		for _, rt := range compiled {
			params, ok := matchRoute(rt, path)
			if !ok {
				continue
			}
			pathMatched = true
			handler, ok := rt.methods[method]
			if !ok {
				continue
			}
			// assoc :path-params into the request and dispatch.
			reqHM := req.(HashMap)
			merged := make(map[MalType]MalType, len(reqHM.Items)+1)
			maps.Copy(merged, reqHM.Items)
			merged[kw("path-params")] = HashMap{Items: params}
			return Apply(ctx, handler, []MalType{HashMap{Items: merged}})
		}
		if pathMatched {
			return statusResponse(http.StatusMethodNotAllowed, "405 method not allowed"), nil
		}
		return statusResponse(http.StatusNotFound, "404 not found"), nil
	}
	return Func{Fn: fn}, nil
}

func mustGet(m MalType, key string) MalType {
	v, _ := hget(m, key)
	return v
}

// --- JWT -----------------------------------------------------------------

// jwksCache holds one auto-refreshing JWKS key set per URL, so verifying
// tokens does not refetch the key set on every request.
var (
	jwksMu    sync.Mutex
	jwksCache = map[string]keyfunc.Keyfunc{}
)

func jwksFor(url string) (keyfunc.Keyfunc, error) {
	jwksMu.Lock()
	defer jwksMu.Unlock()
	if k, ok := jwksCache[url]; ok {
		return k, nil
	}
	k, err := keyfunc.NewDefault([]string{url})
	if err != nil {
		return nil, err
	}
	jwksCache[url] = k
	return k, nil
}

// webVerifyJWT verifies a bearer token against a JWKS and the configured
// issuer/audience, returning its claims as a hash-map. The config map
// accepts :jwks-uri (required), :issuer and :audience (optional, checked
// when present), and :algorithms (defaults to RS256/RS384/RS512 and the
// ES equivalents — what Keycloak issues).
func webVerifyJWT(token string, config MalType) (MalType, error) {
	jwksURI := hgetStr(config, "jwks-uri", "")
	if jwksURI == "" {
		return nil, errors.New("verify-jwt: config needs :jwks-uri")
	}
	k, err := jwksFor(jwksURI)
	if err != nil {
		return nil, fmt.Errorf("verify-jwt: loading JWKS: %w", err)
	}
	algs := []string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}
	if v, ok := hget(config, "algorithms"); ok {
		if slc, err := GetSlice(v); err == nil {
			algs = algs[:0]
			for _, a := range slc {
				algs = append(algs, toStr(a))
			}
		}
	}
	opts := []jwt.ParserOption{jwt.WithValidMethods(algs)}
	if iss := hgetStr(config, "issuer", ""); iss != "" {
		opts = append(opts, jwt.WithIssuer(iss))
	}
	if aud := hgetStr(config, "audience", ""); aud != "" {
		opts = append(opts, jwt.WithAudience(aud))
	}
	parsed, err := jwt.Parse(strings.TrimSpace(token), k.Keyfunc, opts...)
	if err != nil {
		return nil, fmt.Errorf("verify-jwt: %w", err)
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("verify-jwt: unexpected claims type")
	}
	// Round-trip through JSON so nested claims convert uniformly.
	raw, err := json.Marshal(map[string]any(claims))
	if err != nil {
		return nil, err
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil, err
	}
	return jsonToMal(generic), nil
}

// Load registers the web builtins. The Ring response helpers and
// middleware are added by the header (see nsweb.Load).
func Load(env EnvType) {
	call.CallOverrideFN(env, "web-serve", webServe)
	call.CallOverrideFN(env, "web-encode-json", webEncodeJSON)
	call.Doc(env, "web-encode-json", "[value]",
		"Encodes Lisp data as JSON for an HTTP response: keyword keys and values become plain strings (:id → \"id\"), as core json-encode also does.")
	call.Doc(env, "web-serve", "[config]",
		"Starts an HTTP(S) server and blocks until interrupted. config is a hash-map: :handler (a Ring handler fn), :port or :addr, and optional :tls {:cert :key :client-ca :client-auth} for HTTPS/mTLS.")
	call.CallOverrideFN(env, "web-router", webRouter)
	call.Doc(env, "web-router", "[routes]",
		"Returns a Ring handler that dispatches on method and path. routes is a vector of [\"/path/:param\" {:get handler :post handler}] pairs; matched params appear under the request's :path-params.")
	call.CallOverrideFN(env, "web-verify-jwt", webVerifyJWT)
	call.Doc(env, "web-verify-jwt", "[token config]",
		"Verifies a JWT against a JWKS and returns its claims as a hash-map. config: :jwks-uri (required), :issuer, :audience, :algorithms (defaults to Keycloak's RS/ES set).")
}
