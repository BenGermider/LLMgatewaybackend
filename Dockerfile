# Multi-stage build for Go application
FROM golang:latest AS builder

# Set working directory
WORKDIR /app

# Copy go mod file
COPY go.mod ./

# Download dependencies (this will create go.sum if needed)
RUN go mod download

# Copy source code
COPY src/ ./src/

# Build the application
# Adjust the build command based on your main package location
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./src

# Production stage
FROM alpine:latest

# Create non-root user for security
RUN adduser -D -s /bin/sh appuser

WORKDIR /home/appuser

# Copy binary from builder stage
COPY --from=builder /app/main .

# Copy configuration files (adjust as needed)
COPY keys.json usage.json ./

# Change ownership to non-root user
RUN chown -R appuser:appuser /home/appuser

# Switch to non-root user
USER appuser

# Expose port (adjust based on your application)
EXPOSE 8080

# Run the application
CMD ["./main"]