# syntax=docker/dockerfile:1
FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go install github.com/a-h/templ/cmd/templ@v0.3.924
COPY . .
RUN ~/go/bin/templ generate ./internal/dashboard/views/
RUN CGO_ENABLED=0 go build -o golden-gate ./cmd/main.go

FROM alpine:3.18
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata && mkdir -p /data
COPY --from=build /app/golden-gate ./golden-gate
COPY configs ./configs

# Persisted SQLite + service.json live in /data; mount a volume here in
# production so both survive redeploys.
VOLUME ["/data"]
ENV DB_PATH=/data/golden_gate.db
ENV CONFIG_PATH=/data/service.json
EXPOSE 8080
CMD ["./golden-gate"]
