# Build stage
FROM golang:1.25-alpine AS builder

# Install build dependencies required for go-sqlite3
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
# CGO_ENABLED=1 is required for go-sqlite3
RUN CGO_ENABLED=1 go build -o soap-journal ./cmd/server

# Final stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite

# Copy binary from builder
COPY --from=builder /app/soap-journal .

# Expose port
EXPOSE 8080

# Set environment variable for port
ENV PORT=8080

# Create directory for database
RUN mkdir -p /data

# Run the application
CMD ["./soap-journal"]
