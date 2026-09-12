FROM golang:1.26.8-bookworm@sha256:9fdc884aacc3bec89b20ffc69f4bb369c78210e3e4f600387b5128b12c199f81 AS builder
WORKDIR /workspace
ENV GOTOOLCHAIN=local
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -mod=readonly -trimpath -ldflags="-s -w" -o /manager ./cmd

FROM gcr.io/distroless/static:nonroot@sha256:1c2c046bc09ed40fad370b599a0b1ae7987f55b01e247cf27a7c27cd97e5bbc7
ARG VERSION=0.0.0-bootstrap
ARG REVISION=unknown
LABEL org.opencontainers.image.title="AI Workload Control Plane" \
      org.opencontainers.image.description="Namespaced AIWorkload operator bootstrap" \
      org.opencontainers.image.source="https://github.com/erayyilmmaz/ai-workload-control-plane" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version=${VERSION} \
      org.opencontainers.image.revision=${REVISION}
COPY --from=builder /manager /manager
USER 65532:65532
ENTRYPOINT ["/manager"]
