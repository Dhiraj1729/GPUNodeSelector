FROM --platform=$BUILDPLATFORM golang:1.27.1 AS builder
ARG TARGETOS=linux
ARG TARGETARCH=arm64
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd/mock-inference/ ./cmd/mock-inference/
COPY internal/mockinference/ ./internal/mockinference/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /mock-inference ./cmd/mock-inference

FROM scratch AS mock-inference
ARG BUILD_VERSION=dev
LABEL org.opencontainers.image.version=$BUILD_VERSION
COPY --from=builder /mock-inference /mock-inference
USER 65532:65532
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/mock-inference"]

FROM --platform=$BUILDPLATFORM golang:1.27.1 AS plugin-builder
ARG TARGETOS=linux
ARG TARGETARCH=arm64
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/fake-device-plugin/ ./cmd/fake-device-plugin/
COPY internal/fakegpu/ ./internal/fakegpu/
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /fake-device-plugin ./cmd/fake-device-plugin

FROM scratch AS fake-device-plugin
ARG BUILD_VERSION=dev
LABEL org.opencontainers.image.version=$BUILD_VERSION
COPY --from=plugin-builder /fake-device-plugin /fake-device-plugin
# UID 0 can create sockets in kubelet's root-owned directory; no capabilities needed.
USER 0:0
ENTRYPOINT ["/fake-device-plugin"]

FROM --platform=$BUILDPLATFORM golang:1.27.1 AS operator-builder
ARG TARGETOS=linux
ARG TARGETARCH=arm64
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY api/ ./api/
COPY internal/controller/ ./internal/controller/
COPY internal/workload/ ./internal/workload/
COPY cmd/main.go ./cmd/main.go
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /operator ./cmd

FROM scratch AS operator
ARG BUILD_VERSION=dev
LABEL org.opencontainers.image.version=$BUILD_VERSION
COPY --from=operator-builder /operator /operator
USER 65532:65532
ENTRYPOINT ["/operator"]
