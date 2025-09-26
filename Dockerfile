# Multi-stage build for Go application
FROM golang:latest AS builder

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum
COPY go.mod ./

# Download dependencies
RUN go mod download

# Copy the rest of the code
COPY . .

# Build the Go binary
# Main package is in cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/server

# Production stage
FROM alpine:latest

# Create non-root user
RUN adduser -D -s /bin/sh appuser

WORKDIR /home/appuser

# Copy binary from builder
COPY --from=builder /app/main .

# Copy configuration files
COPY keys.json usage.json ./

# Change ownership
RUN chown -R appuser:appuser /home/appuser

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Run the application
CMD ["./main"]