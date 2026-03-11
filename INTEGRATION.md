# Authentication System - Integration Guide

This guide explains how to integrate and run the new authentication system.

## Project Structure

```
messenger-app-backend/
├── backend/
│   ├── main.go      # Server entry point, routes, CORS
│   ├── db.go        # MySQL connection, users table
│   ├── auth.go      # Register, Login handlers, JWT
│   └── middleware.go # JWT auth middleware
├── frontend/
│   ├── index.html   # Landing page, profile (when logged in)
│   ├── login.html   # Login + register toggle
│   ├── register.html# Standalone registration
│   └── style.css    # Shared styles
├── go.mod
└── go.sum
```

## Prerequisites

1. **MySQL** installed and running
2. **Go 1.22+**
3. A database named `messenger` (or adjust `DATABASE_URL`)

## Step 1: Create the Database

```sql
-- In MySQL client (mysql, MySQL Workbench, etc.)
CREATE DATABASE messenger;
```

## Step 2: Set Environment Variables (Optional)

```bash
# Default: root:YourPassword@tcp(localhost:3306)/messenger
# Format: USER:PASSWORD@tcp(HOST:PORT)/DBNAME
export DATABASE_URL="user:password@tcp(localhost:3306)/messenger"
```

# For production, also set a secure JWT secret (edit backend/auth.go or add env var)
```

## Step 3: Run the Server

```bash
# From project root - IMPORTANT: run from project root so frontend/ is found
cd messenger-app-backend
go run ./backend
```

The server starts at `http://localhost:8080`.

## Step 4: Verify Endpoints

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | / | No | Serves index.html |
| GET | /login | No | Serves login.html |
| GET | /register.html | No | Serves register.html |
| POST | /register | No | Create account |
| POST | /login | No | Login, returns JWT |
| GET | /profile | Yes (Bearer) | Returns user info |
| GET | /ping | No | Health check |

## Step 5: Test the Flow

1. **Register**: Visit `/register.html` or click "Create Account" on login page
2. **Login**: Enter email and password, submit
3. **Profile**: After login, you're redirected to `/` which fetches `/profile` with the JWT
4. **Logout**: Click Logout to clear the token

## API Examples

### Register
```bash
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","email":"alice@example.com","password":"secret123"}'
```

### Login
```bash
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret123"}'
# Returns: {"token":"...","user_id":1,"username":"alice","email":"alice@example.com"}
```

### Profile (Protected)
```bash
curl http://localhost:8080/profile \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## Migrating from the Old Server

If you have an existing `main.go` at the project root:

1. The new server is in `backend/` - use `go run ./backend` instead of `go run .`
2. Remove or rename the old `main.go` to avoid confusion
3. The frontend now uses absolute paths (`/style.css`, `/asserts/...`) - ensure the server serves these

## Production Checklist

- [ ] Set `DATABASE_URL` via environment
- [ ] Set a strong JWT secret in `backend/auth.go` (or via env)
- [ ] Use HTTPS
- [ ] Restrict CORS origins in `enableCORS`
- [ ] Add rate limiting for login/register
