FROM golang:1.24-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/social-network ./server.go

FROM alpine:3.21

WORKDIR /app

RUN apk add --no-cache ca-certificates

COPY --from=build /bin/social-network /usr/local/bin/social-network
COPY --from=build /app/pkg/db/migrations ./pkg/db/migrations

RUN mkdir -p /app/uploads

ENV HOST=0.0.0.0
ENV PORT=8080
ENV MIGRATIONS_DIR=/app/pkg/db/migrations/postgres
ENV UPLOADS_DIR=/app/uploads
ENV PUBLIC_BASE_URL=http://localhost:8080

EXPOSE 8080

CMD ["social-network"]
