# Multi-stage build for Go application
FROM golang:latest AS builder

WORKDIR /app
COPY go.mod ./

RUN go mod download
COPY . .

# Execution
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/server

# For production - lightweight, safe (non-root) container
FROM alpine:latest

# New user - not root
RUN adduser -D -s /bin/sh appuser

WORKDIR /home/appuser

# Load executable
COPY --from=builder /app/main .

# Databases
COPY keys.json usage.json ./

# Give user ownership
RUN chown -R appuser:appuser /home/appuser

USER appuser

EXPOSE 8080

# Run the application
CMD ["./main"]