# Build Stage
FROM golang:1.16.8-alpine3.13 AS builder

# Tool versions
ARG PROTODOC_VERSION=1.3.2
ARG GRPC_GATEWAY_VERSION=1.16.0
ARG REGEN_GOGOPROTO_VERSION=0.3.0
ARG REGEN_PROTOBUF_VERSION=1.3.2-alpha.regen.4
ARG BUF_VERSION=0.30.0

# Install build dependencies
RUN apk --no-cache add \
    curl \
    git \
    libc6-compat \
    make \
    upx

# Setup protobuf includes
RUN go get -d github.com/gogo/protobuf/gogoproto && \
    mkdir -p /usr/include/google/protobuf/ && \
    mv /go/src/github.com/gogo/protobuf/protobuf/google/protobuf/empty.proto \
       /go/src/github.com/gogo/protobuf/protobuf/google/protobuf/descriptor.proto \
       /usr/include/google/protobuf/

# Install gRPC gateway tools
RUN GO111MODULE=on go get \
    github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@v${GRPC_GATEWAY_VERSION} \
    github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger@v${GRPC_GATEWAY_VERSION} && \
    mv /go/bin/protoc-gen-* /usr/local/bin/

# Install regen protobuf tools
WORKDIR /work
COPY go.mod go.sum* ./
RUN GO111MODULE=on go mod init temp-module && \
    go mod edit -replace github.com/gogo/protobuf=github.com/regen-network/protobuf@v${REGEN_PROTOBUF_VERSION} && \
    go get github.com/regen-network/cosmos-proto/protoc-gen-gocosmos@v${REGEN_GOGOPROTO_VERSION} && \
    mv /go/bin/protoc-gen-gocosmos* /usr/local/bin/

# Install additional protobuf tools
RUN GO111MODULE=on go get \
    github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@v${PROTODOC_VERSION} \
    github.com/bufbuild/buf/cmd/buf@v${BUF_VERSION} && \
    mv /go/bin/protoc-gen-doc /go/bin/buf /usr/local/bin/

# Compress binaries
RUN upx --lzma /usr/local/bin/*

# Final Stage
FROM golang:1.19-alpine

# Environment setup
ENV LD_LIBRARY_PATH=/lib64:/lib
WORKDIR /work

# Install runtime dependencies
RUN apk --no-cache add \
    ca-certificates \
    curl \
    git \
    libc6-compat \
    make \
    protoc

# Copy built tools and includes from builder
COPY --from=builder /usr/local/bin/ /usr/local/bin/
COPY --from=builder /usr/include/ /usr/include/

# Set permissions
RUN chmod -R 755 /usr/include

# Health check
HEALTHCHECK --interval=30s --timeout=3s \
    CMD protoc --version || exit 1

# Default command
CMD ["protoc", "--version"]

# Labels
LABEL maintainer="Wasmd Team" \
      description="Protocol buffer and gRPC tools for Wasmd" \
      version="1.0" \
      org.opencontainers.image.source="https://github.com/CosmWasm/wasmd"
