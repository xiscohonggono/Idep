# Build Stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Download dependencies first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and compile statically
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/server ./cmd/server

# Production Stage
FROM alpine:3.20

WORKDIR /app

# Install root certs for TLS/HTTPS calls (e.g. Supabase connection)
RUN apk --no-cache add ca-certificates tzdata

# Copy compiled binary from builder
COPY --from=builder /app/bin/server /app/server

EXPOSE 8080

CMD ["/app/server"]
