package web_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/jig/lisp/lib/web"
	. "github.com/jig/lisp/types"
)

// TestVerifyJWT covers the Keycloak-style path end to end: a JWKS served
// over HTTP, an RS256 token signed with the matching private key, and
// issuer/audience validation, decoded into a claims hash-map.
func TestVerifyJWT(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "test-key-1"
	jwks := jwksJSON(t, &key.PublicKey, kid)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write(jwks)
	}))
	defer srv.Close()

	const issuer = "https://kc.example/realms/test"
	const audience = "my-api"
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":                "user-123",
		"iss":                issuer,
		"aud":                audience,
		"exp":                time.Now().Add(time.Hour).Unix(),
		"preferred_username": "alice",
		"realm_access":       map[string]any{"roles": []string{"admin", "user"}},
	})
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}

	config := HashMap{Items: map[MalType]MalType{
		NewKeyword("jwks-uri"): srv.URL,
		NewKeyword("issuer"):   issuer,
		NewKeyword("audience"): audience,
	}}

	claims, err := web.VerifyJWT(signed, config)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	hm, ok := claims.(HashMap)
	if !ok {
		t.Fatalf("claims not a hash-map: %T", claims)
	}
	if got := hm.Items[NewKeyword("sub")]; got != "user-123" {
		t.Fatalf(":sub = %v", got)
	}
	if got := hm.Items[NewKeyword("preferred_username")]; got != "alice" {
		t.Fatalf(":preferred_username = %v", got)
	}
	// nested claims convert too
	ra, ok := hm.Items[NewKeyword("realm_access")].(HashMap)
	if !ok {
		t.Fatalf("realm_access not a hash-map")
	}
	roles, ok := ra.Items[NewKeyword("roles")].(Vector)
	if !ok || len(roles.Val) != 2 || roles.Val[0] != "admin" {
		t.Fatalf("roles = %v", ra.Items[NewKeyword("roles")])
	}

	// a wrong audience is rejected
	badCfg := HashMap{Items: map[MalType]MalType{
		NewKeyword("jwks-uri"): srv.URL,
		NewKeyword("audience"): "someone-else",
	}}
	if _, err := web.VerifyJWT(signed, badCfg); err == nil {
		t.Fatal("expected audience mismatch to fail")
	}
}

// jwksJSON builds a minimal JWKS containing one RSA public key.
func jwksJSON(t *testing.T, pub *rsa.PublicKey, kid string) []byte {
	t.Helper()
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	eBytes := exponentBytes(pub.E)
	e := base64.RawURLEncoding.EncodeToString(eBytes)
	jwk := map[string]any{
		"keys": []map[string]any{{
			"kty": "RSA", "use": "sig", "alg": "RS256", "kid": kid, "n": n, "e": e,
		}},
	}
	b, err := json.Marshal(jwk)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// exponentBytes returns the minimal big-endian bytes of a small positive int (the
// RSA public exponent, typically 65537).
func exponentBytes(v int) []byte {
	var out []byte
	for v > 0 {
		out = append([]byte{byte(v & 0xff)}, out...)
		v >>= 8
	}
	return out
}
