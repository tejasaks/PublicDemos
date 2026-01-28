# Build stage
FROM golang:1.22-alpine AS builder

WORKDIR /workspace

# Copy go mod files
COPY go.mod go.mod
COPY go.sum go.sum

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY internal/ internal/

# Build the operator
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o mssql-operator ./cmd/mssql-operator

# Runtime stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /

COPY --from=builder /workspace/mssql-operator .

USER 65532:65532

ENTRYPOINT ["/mssql-operator"]
