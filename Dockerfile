# syntax=docker/dockerfile:1
FROM golang:1.21-alpine AS build
WORKDIR /app
COPY . .
RUN go mod download
RUN go install github.com/a-h/templ/cmd/templ@latest
RUN ~/go/bin/templ generate ./internal/dashboard/views/
RUN go build -o golden-gate ./cmd/main.go

FROM alpine:3.18
WORKDIR /app
COPY --from=build /app/golden-gate ./golden-gate
COPY configs ./configs
EXPOSE 8080
CMD ["./golden-gate"] 