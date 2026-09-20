# syntax=docker/dockerfile:1
ARG GO_IMAGE=golang:1.27.1-bookworm@sha256:69a7b9788769bec032d238959b61854e9ae87f57be9029ec04e9885fabf99195
FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS source
WORKDIR /src
# Generated bindings are committed source; protoc is only needed to regenerate them.
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .

FROM source AS test
RUN go test -mod=readonly -race -count=1 ./... && go vet -mod=readonly ./...

FROM source AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-$(go env GOARCH)} \
    go build -mod=readonly -trimpath -ldflags="-s -w -buildid= -X main.version=${VERSION}" -o /out/server ./cmd/server && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-$(go env GOARCH)} \
    go build -mod=readonly -trimpath -ldflags="-s -w -buildid=" -o /out/replicactl ./cmd/replicactl && \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-$(go env GOARCH)} \
    go build -mod=readonly -trimpath -ldflags="-s -w -buildid=" -o /out/probe ./cmd/probe

# The statically linked binaries need CA roots, but no shell, compiler or package manager.
FROM scratch AS runtime
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/ /
USER 65532:65532
EXPOSE 8443 8081
ENTRYPOINT ["/server"]
