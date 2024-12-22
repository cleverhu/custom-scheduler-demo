# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

RUN go env -w GOPROXY=https://goproxy.cn,direct
# Cache dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -a -o custom-scheduler cmd/scheduler/main.go

# Runtime stage
FROM alpine:3.18

WORKDIR /
COPY --from=builder /workspace/custom-scheduler .

USER 65532:65532

ENTRYPOINT ["/custom-scheduler"] 