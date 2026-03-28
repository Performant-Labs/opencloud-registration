# Stage 1: build
FROM golang:1.22-bookworm AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux \
    go build -ldflags="-s -w" -o /server ./cmd/server

# Stage 2: runtime
FROM debian:bookworm-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates wget \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /server .

VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/app/server"]
