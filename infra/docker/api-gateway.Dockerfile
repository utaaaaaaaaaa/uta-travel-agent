# Go API Gateway Dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/
COPY proto/ proto/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /api-gateway ./cmd/api-gateway

# Final stage
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /api-gateway /app/api-gateway

# Copy agent templates
COPY agent-templates/ /app/agent-templates/

# Expose port
EXPOSE 8080

# Run
CMD ["/app/api-gateway"]