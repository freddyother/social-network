# Frontend

`Vue 3` SPA separated from the Go backend.

## What This Foundation Includes

- `Vue 3` with `Vite`
- `Vue Router` for primary navigation
- an HTTP client pointing to the backend at `http://localhost:8080/api/v1`
- a base layout for feed, profiles, groups, notifications, and chat
- ready to work with `CORS` and session cookies
- working authentication flow for register, login, logout, and current session bootstrap
- a Dockerfile for static build and deployment

## Environment Variables

```env
VITE_API_BASE_URL=http://localhost:8080/api/v1
```

## Local Run

```bash
npm install
npm run dev
```

## Scripts

- `npm run dev`
- `npm run build`
- `npm run preview`

## Included Routes

- `/`
- `/feed`
- `/profile/:handle?`
- `/groups`
- `/notifications`
- `/chat`
- `/login`
- `/register`

## Natural Next Step

Build the first authenticated product modules on top of the new session flow: feed, profile privacy, followers, groups, notifications, and WebSocket chat.
