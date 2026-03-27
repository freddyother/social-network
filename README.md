# Social Network

Initial foundation split into two projects:

- `backend/`: Go API with `PostgreSQL`, SQL migrations, CORS, and Docker.
- `frontend/`: `Vue 3` SPA with `Vue Router`, an HTTP client, and Docker.

Current status:

- health and meta endpoints
- SQL schema for users, follows, posts, groups, notifications, and chat
- end-to-end authentication with registration, login, logout, session cookies, and `GET /auth/me`

Vue 3 is a strong fit for this project because the social network needs a highly interactive SPA with a lot of shared state, reactive screens, forms, notifications, chat, and real-time updates.

## Structure

```text
.
├── backend
│   ├── README.md
│   ├── server.go
│   ├── internal
│   └── pkg
├── frontend
│   ├── README.md
│   ├── src
│   └── public
└── docker-compose.yml
```

## Quick Start

```bash
docker compose up --build
```

Services:

- Frontend: `http://localhost:4173`
- Backend: `http://localhost:8080/api/v1`
- PostgreSQL: `localhost:5432`

## Documentation

- See `backend/README.md`
- See `frontend/README.md`
