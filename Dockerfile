# Stage 1: Build binary using pure-Go (CGO_ENABLED=0)
FROM golang:alpine AS builder

WORKDIR /src

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code and build statically linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /bin/cli ./cmd/cli

# Stage 2: Minimal runtime image
FROM alpine:3.21

# Install ca-certificates and sqlite CLI for maintenance/inspection
RUN apk --no-cache add ca-certificates sqlite tzdata

# Create dedicated non-root user and group (UID/GID 1000)
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -s /bin/sh -D appuser

# Create persistent data directory and grant permissions
RUN mkdir -p /data && chown -R appuser:appgroup /data

USER appuser:appgroup
WORKDIR /home/appuser

COPY --from=builder /bin/cli /bin/cli

VOLUME ["/data"]

ENTRYPOINT ["/bin/cli"]
