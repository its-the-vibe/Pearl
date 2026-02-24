# Build stage
FROM golang:1.24 AS builder

WORKDIR /build

# Copy dependency files and download modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build a statically linked binary
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o pearl .

# Runtime stage – minimal scratch image
FROM scratch

COPY --from=builder /build/pearl /pearl
# Include CA certificates for TLS connections to Google APIs
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["/pearl"]
