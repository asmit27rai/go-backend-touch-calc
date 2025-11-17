# Build stage
FROM golang:1.22-alpine AS builder

# Set working directory
WORKDIR /app

# Install dependencies
RUN apk add --no-cache git build-base

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Debug: List template files before build
RUN echo "=== CHECKING FILES BEFORE BUILD ===" &&     find web -name "*.html" -type f || echo "No HTML files found" &&     ls -la web/templates/ || echo "No templates directory" &&     ls -la web/static/ || echo "No static directory"

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o main cmd/server/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates and tools for health checks
RUN apk --no-cache add ca-certificates curl wget

# Create non-root user FIRST
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Set proper working directory and ownership
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/main ./main

# Copy web files with proper structure
COPY --from=builder /app/web ./web
COPY --from=builder /app/webappTemplates ./webappTemplates

# Create proper CSS directory structure and move CSS files
RUN mkdir -p /app/web/static/css &&     mv /app/web/static/js/*.css /app/web/static/css/ 2>/dev/null || true

# Debug: Verify files were copied correctly
RUN echo "=== VERIFYING COPIED FILES ===" &&     ls -la web/templates/ &&     ls -la web/static/css/ &&     ls -la web/static/js/ &&     echo "=== FILES CHECK COMPLETE ==="

# Set ownership of ALL files to appuser
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3     CMD curl -f http://localhost:8080/health || exit 1

# Run the application
CMD ["./main"]
