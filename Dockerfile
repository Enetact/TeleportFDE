# syntax=docker/dockerfile:1
ARG GO_VERSION=1.26.8
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
WORKDIR /src
# Run make prepare first; its real go.sum and generated protobuf code are required.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -mod=readonly -trimpath -ldflags="-s -w -buildid= -X main.version=${VERSION}" -o /out/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -mod=readonly -trimpath -ldflags="-s -w -buildid=" -o /out/replicactl ./cmd/replicactl && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -mod=readonly -trimpath -ldflags="-s -w -buildid=" -o /out/probe ./cmd/probe
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/ /
USER 65532:65532
EXPOSE 8443 8081
ENTRYPOINT ["/server"]
