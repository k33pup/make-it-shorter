FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate proto files
RUN apk add --no-cache protobuf-dev
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.31.0
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.3.0
RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    api/proto/*.proto

# Update dependencies after proto generation
RUN go mod tidy

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./services/gateway

FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates

COPY --from=builder /app/gateway .
COPY --from=builder /app/web /app/web

EXPOSE 8080

CMD ["./gateway"]
