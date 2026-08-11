# Multi-stage Dockerfile for the Unsample demo services.
# Builds all 3 services from a single image, controlled by SERVICE build arg.
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Copy the SDK module first (for caching).
COPY sdk/go/ /app/sdk/go/

# Copy the demo app module.
COPY examples/demo-app/ /app/examples/demo-app/

WORKDIR /app/examples/demo-app

# Download dependencies.
RUN go mod download

# Build arg to select which service to build.
ARG SERVICE=gateway

RUN CGO_ENABLED=0 GOOS=linux go build -o /service ./${SERVICE}/

# --- Runtime ---
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=builder /service /service

EXPOSE 8080

ENTRYPOINT ["/service"]
