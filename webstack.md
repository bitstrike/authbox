# Web Stack

Single Go binary serves both API and frontend on the same port.

## Layout

```
internal/
  web/
    api/         # JSON REST handlers for /api/v1/* and /internal/*
    frontend/    # Server-rendered HTML for all other routes
      templates/ # Go html/template files
      static/    # CSS, icons, minimal JS
```

## Frontend

- Go `html/template` for server-rendered HTML
- HTMX for dynamic interactions (no full page reloads)
- Tailwind CSS (or Pico CSS) for styling
- Light and dark theme (system preference + manual toggle)
- No JavaScript framework, no Node.js build step

## API

- `chi` router
- JSON request/response
- OAuth2 bearer token authentication
- Role-based middleware

## Libraries

- `github.com/go-chi/chi/v5` - routing
- `github.com/go-ldap/ldap/v3` - LDAP client
- `modernc.org/sqlite` - SQLite (pure Go, no CGO)
- `github.com/coreos/go-oidc/v3` - OIDC verification
- `golang.org/x/crypto/ssh` - SSH cert signing
- `golang.org/x/oauth2` - OAuth2 flows
