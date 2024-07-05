FROM golang:1.22 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
COPY vendor vendor

# Copy the go source
COPY main.go main.go

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o network-topology-discovery main.go

FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt update && apt install -y wget infiniband-diags slurmctld gawk && apt clean && rm -rf /var/lib/apt/lists/*
COPY slurm/slurm.conf /etc/slurm-llnl/slurm.conf
COPY slurm/slurmibtopology.sh /usr/local/bin/slurmibtopology.sh
RUN chmod +x /usr/local/bin/slurmibtopology.sh

WORKDIR /
COPY --from=builder /workspace/network-topology-discovery .

ENTRYPOINT ["/network-topology-discovery"]
