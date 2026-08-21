# syntax=docker/dockerfile:1

# Build stage
FROM --platform=$BUILDPLATFORM golang:1.27.0-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /pearl ./cmd/pearl

# Runtime stage (distroless)
FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=builder /pearl /pearl

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/pearl"]
