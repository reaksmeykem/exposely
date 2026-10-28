package main

import "testing"

func TestIsSensitiveStaticPath(t *testing.T) {
	blocked := []string{
		"/.env",
		"/.git/config",
		"/sub/.htpasswd",
		"/app.js.map",
		"/dir/.hidden/file.html",
	}
	for _, p := range blocked {
		if !isSensitiveStaticPath(p) {
			t.Fatalf("expected %q to be blocked", p)
		}
	}

	allowed := []string{
		"/",
		"/index.html",
		"/assets/app.js",
		"/.well-known/acme-challenge/token",
		"/images/logo.png",
	}
	for _, p := range allowed {
		if isSensitiveStaticPath(p) {
			t.Fatalf("expected %q to be served", p)
		}
	}
}
