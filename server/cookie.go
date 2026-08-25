package server

import (
	"net/http"
	"strings"
)

// IsSecureRequest reports whether the request arrived over HTTPS, either
// directly (r.TLS != nil) or through a reverse proxy that terminates TLS
// and sets the X-Forwarded-Proto header.
func IsSecureRequest(r *http.Request) bool {
	return r.TLS != nil ||
		strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// secureCookieAttr reports whether the Secure attribute should be set on
// authentication cookies for the given request. Secure is enabled by
// default; insecureCookie disables it so that development over plain HTTP
// keeps working. Even then cookies stay Secure when the request itself is
// over HTTPS.
func secureCookieAttr(insecureCookie bool, r *http.Request) bool {
	return !insecureCookie || IsSecureRequest(r)
}
