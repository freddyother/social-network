# Backend

Base Go API for the social network, separated from the Vue 3 frontend and prepared to communicate through `CORS`.

Important:
This backend uses `PostgreSQL`

## What This Foundation Includes

- `server.go` as the entry point
- HTTP server with graceful shutdown
- `CORS` middleware ready for cookies and sessions
- `PostgreSQL` connection setup
- self-managed SQL migrations applied on startup
- initial endpoints: `GET /api/v1/health` and `GET /api/v1/meta`
- authentication endpoints with secure password hashing and session cookies
- feed endpoints with multi-image uploads, follower-only posts, public/private accounts, and follow requests
- a structure ready to grow into auth, followers, posts, groups, notifications, and chat

## Structure

```text
backend
├── server.go
├── internal
│   ├── app
│   ├── config
│   └── http
└── pkg
    └── db
        ├── migrations
        │   └── postgres
        └── postgres
```

## Environment Variables

```env
APP_ENV=development
HOST=0.0.0.0
PORT=8080
DATABASE_URL=postgres://postgres:postgres@localhost:5432/social_network?sslmode=disable
MIGRATIONS_DIR=./pkg/db/migrations/postgres
UPLOADS_DIR=./uploads
PUBLIC_BASE_URL=http://localhost:8080
CORS_ALLOWED_ORIGINS=http://localhost:5173,http://127.0.0.1:5173,http://localhost:4173,http://127.0.0.1:4173
SESSION_COOKIE_NAME=social_network_session
SESSION_TTL_HOURS=720
SESSION_COOKIE_SECURE=false
```

## Local Run

```bash
go mod tidy
go run ./server.go
```

## Base Endpoints

- `GET /api/v1/health`
- `GET /api/v1/meta`
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/logout`
- `GET /api/v1/auth/me`
- `GET /api/v1/posts`
- `POST /api/v1/posts`
- `GET /api/v1/users/discover`
- `POST /api/v1/users/{userID}/follow`
- `GET /api/v1/follow-requests`
- `POST /api/v1/follow-requests/{requestID}/accept`
- `POST /api/v1/follow-requests/{requestID}/decline`
- `PATCH /api/v1/users/me/profile-visibility`

## Recommended Next Layers

- `internal/users` for public and private profiles
- `internal/posts` for feed, comments, and images
- `internal/groups` for groups, invitations, and events
- `internal/chat` for private and group WebSockets
- `internal/notifications` for the global inbox and system events
