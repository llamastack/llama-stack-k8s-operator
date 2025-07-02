# Build the manager binary
ARG GOLANG_VERSION=1.23

FROM registry.access.redhat.com/ubi8/go-toolset:${GOLANG_VERSION} as builder
ARG TARGETOS=linux
ARG TARGETARCH
ARG CGO_ENABLED=0

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build the manager binary
USER root

# GOARCH is intentionally left empty to automatically detect the host architecture
# This ensures the binary matches the platform where image-build is executed
RUN CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/controllers/manifests ./manifests/
USER 1001

ENTRYPOINT ["/manager"]
