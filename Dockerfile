# syntax=docker/dockerfile:1

# Build stage
FROM golang:1.26.0 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" -o /pearl ./cmd/pearl

# Runtime stage – minimal scratch image
FROM scratch

COPY --from=builder /pearl /pearl
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

EXPOSE 8080

ENTRYPOINT ["/pearl"]
