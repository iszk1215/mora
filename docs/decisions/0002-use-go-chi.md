# ADR 0002: Use go-chi for HTTP routing

## Status

Accepted

## Context

Mora needs an HTTP router for its REST API and frontend serving. The router must support middleware composition, URL parameters, and sub-route mounting.

## Decision

Use `github.com/go-chi/chi/v5` as the HTTP router.

## Consequences

### Positive

- stdlib-compatible: `http.Handler` and `http.HandlerFunc` everywhere, no framework lock-in
- Lightweight: minimal dependency tree, easy to understand
- Rich middleware ecosystem (`chi/middleware` for logging, recovery, compression)
- Sub-route grouping via `r.Route()` and `r.Mount()` matches Mora's modular handler design
- URL parameters via `chi.URLParam()` are straightforward

### Negative

- No built-in request validation (must handle manually)
- No code generation for type-safe routes (acceptable for this project size)

### Alternatives considered

- `gorilla/mux`: more features but heavier, less stdlib-aligned
- `net/http` + manual routing: too verbose for the number of endpoints
