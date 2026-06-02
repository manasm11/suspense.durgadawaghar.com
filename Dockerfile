# Stage 1: Build
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install templ CLI (version pinned to match go.mod)
RUN go install github.com/a-h/templ/cmd/templ@v0.3.977

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN templ generate
RUN CGO_ENABLED=0 go build -o suspense ./cmd/server

# Stage 2: Runtime
FROM alpine:3.21

WORKDIR /app

COPY --from=builder /build/suspense .

RUN mkdir -p /app/data

EXPOSE 8005

CMD ["./suspense", "-db", "/app/data/suspense.db"]
