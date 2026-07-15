package web_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"

	"github.com/jig/lisp"
	"github.com/jig/lisp/env"
	"github.com/jig/lisp/lib/concurrent/nsconcurrent"
	"github.com/jig/lisp/lib/core/nscore"
	"github.com/jig/lisp/lib/coreextended/nscoreextended"
	"github.com/jig/lisp/lib/web"
	"github.com/jig/lisp/lib/web/nsweb"
	"github.com/jig/lisp/types"
)

// newEnv loads core + concurrent + the web namespace.
func newEnv(t *testing.T) types.EnvType {
	t.Helper()
	ns := env.NewEnv()
	if err := nscore.Load(ns); err != nil {
		t.Fatal(err)
	}
	if err := nscore.LoadInput(ns); err != nil {
		t.Fatal(err)
	}
	if err := nsconcurrent.Load(ns); err != nil {
		t.Fatal(err)
	}
	if err := nscoreextended.Load(ns); err != nil {
		t.Fatal(err)
	}
	if err := nsweb.Load(ns); err != nil {
		t.Fatal(err)
	}
	return ns
}

// handlerFrom evaluates src to a Ring handler value.
func handlerFrom(t *testing.T, ns types.EnvType, src string) types.MalType {
	t.Helper()
	ast, err := lisp.READ(src, types.NewCursorFile(t.Name()), ns)
	if err != nil {
		t.Fatal(err)
	}
	v, err := lisp.EVAL(context.Background(), ast, ns)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// ringHandler is exported for tests via the test hook.
func TestServeHTTPRoundTrip(t *testing.T) {
	ns := newEnv(t)
	// A router with a param route, wrapped in the JSON/log/recover stack.
	h := handlerFrom(t, ns, `
	  (-> (web/router
	        [["/hello/:name" {:get (fn [req] (web/json {:hi (get (get req :path-params) :name)
	                                                    :q  (get (get req :query) "n")}))}]
	         ["/boom" {:get (fn [req] (throw "kaboom"))}]])
	      web/wrap-json-body
	      web/wrap-recover)`)

	srv := httptest.NewServer(web.RingHandler(h))
	defer srv.Close()

	// happy path with a path param and a query param
	resp := get(t, srv.Client(), srv.URL+"/hello/world?n=42")
	if resp.status != 200 {
		t.Fatalf("status %d", resp.status)
	}
	if resp.body != `{"hi":"world","q":"42"}` {
		t.Fatalf("body %q", resp.body)
	}
	if ct := resp.header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type %q", ct)
	}

	// wrap-recover turns a thrown error into a 500 JSON body
	boom := get(t, srv.Client(), srv.URL+"/boom")
	if boom.status != 500 {
		t.Fatalf("boom status %d", boom.status)
	}

	// unknown path → router 404
	nf := get(t, srv.Client(), srv.URL+"/nope")
	if nf.status != 404 {
		t.Fatalf("404 status %d", nf.status)
	}
}

// TestServeMTLS checks that a verified client certificate surfaces as
// :mtls in the request and can be promoted to :identity.
func TestServeMTLS(t *testing.T) {
	ns := newEnv(t)
	h := handlerFrom(t, ns, `
	  (-> (fn [req] (web/json {:subject (get (get req :identity) :subject)
	                           :kind    (get (get req :identity) :kind)}))
	      web/wrap-identity)`)

	caCert, caKey := makeCA(t)
	srv := httptest.NewUnstartedServer(web.RingHandler(h))
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	srv.TLS = &tls.Config{ClientCAs: pool, ClientAuth: tls.RequireAndVerifyClientCert}
	srv.StartTLS()
	defer srv.Close()

	clientCert := makeClientCert(t, caCert, caKey, "svc-worker")
	client := srv.Client()
	tr := client.Transport.(*http.Transport)
	tr.TLSClientConfig.Certificates = []tls.Certificate{clientCert}

	resp := get(t, client, srv.URL+"/")
	if resp.status != 200 {
		t.Fatalf("status %d body %q", resp.status, resp.body)
	}
	if resp.body != `{"kind":"mtls","subject":"svc-worker"}` {
		t.Fatalf("identity body %q", resp.body)
	}
}

// --- helpers -------------------------------------------------------------

type response struct {
	status int
	body   string
	header http.Header
}

func get(t *testing.T, c *http.Client, url string) response {
	t.Helper()
	r, err := c.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Body.Close() }()
	b, _ := io.ReadAll(r.Body)
	return response{status: r.StatusCode, body: string(b), header: r.Header}
}

func makeCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert, key
}

func makeClientCert(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: nil}
}
