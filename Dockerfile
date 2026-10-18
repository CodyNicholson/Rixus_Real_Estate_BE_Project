# Stage 1: Build the Go binary
FROM golang:alpine AS builder

WORKDIR /app

ENV GOTOOLCHAIN=auto

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy application source code
COPY . .

# Ensure dependencies are tidy and build statically linked binary
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/server ./cmd/server

# Stage 2: Minimal runtime image
FROM alpine:3.20

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder stage
COPY --from=builder /app/server /app/server

EXPOSE 8080

ENTRYPOINT ["/app/server"]

