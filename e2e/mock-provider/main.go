package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
)

var (
	mockUsername = "testuser"
	mockUserID   = 9999
)

func handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	redirectURI := q.Get("redirect_uri")
	state := q.Get("state")

	if redirectURI == "" {
		http.Error(w, "missing redirect_uri", http.StatusBadRequest)
		return
	}

	code := "mock-auth-code-12345"

	// Ensure redirect_uri ends with / so chi's mounted sub-router matches
	if !strings.HasSuffix(redirectURI, "/") {
		redirectURI += "/"
	}

	sep := "?"
	if strings.Contains(redirectURI, "?") {
		sep = "&"
	}
	callbackURL := fmt.Sprintf("%s%scode=%s&state=%s", redirectURI, sep, code, state)
	http.Redirect(w, r, callbackURL, http.StatusFound)
}

func handleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token":  "mock-access-token-12345",
		"token_type":    "bearer",
		"scope":         "repo",
		"created_at":    1700000000,
	})
}

func handleUser(w http.ResponseWriter, r *http.Request) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "http"
	}
	avatarURL := fmt.Sprintf("%s://%s/avatar.png", scheme, r.Host)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":         mockUserID,
		"login":      mockUsername,
		"username":   mockUsername,
		"full_name":  "Test User",
		"avatar_url": avatarURL,
		"email":      "test@example.com",
	})
}

func handleAvatar(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	// 1x1 green pixel PNG
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
		0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xd8, 0xab, 0xf9, 0xcf,
		0xc0, 0x00, 0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc, 0x33, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
	w.Write(png)
}

func main() {
	port := flag.Int("port", 4001, "Mock OAuth provider port")
	flag.Parse()

	http.HandleFunc("/login/oauth/authorize", handleAuthorize)
	http.HandleFunc("/login/oauth/access_token", handleToken)
	http.HandleFunc("/api/v1/user", handleUser)
	http.HandleFunc("/avatar.png", handleAvatar)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("Mock OAuth provider listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
