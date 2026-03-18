# Messenger App Backend

Fresh rebuild with a minimal Go backend plus static HTML/CSS frontend for auth.

## Features
- Register and login with bcrypt + JWT.
- Protected `/profile` endpoint (Bearer token).
- Health check at `/healthz` and `/ping`.
- Serves `frontend/` (home, login, register) and `/asserts/*` assets.

## Quick start
1) Requirements: Go (>=1.20) and a MySQL instance.
2) Env (optional):
   - `DATABASE_URL` e.g. `user:pass@tcp(localhost:3306)/messenger?parseTime=true`
   - `JWT_SECRET` set to a strong random string.
   - `PORT` (default `8080`).
3) Run migrations: automatically creates a `users` table on start.
4) Start the server: `go run ./backend`
5) Open `http://localhost:8080` for the UI.

## API
- `POST /register` `{email, username, password}`
- `POST /login` `{email, password}` -> `{token, user_id, username, email}`
- `GET /profile` with `Authorization: Bearer <token>`

## Dev helpers
- Format: `gofmt -w backend/*.go`
- Basic compile check: `go test ./...`
