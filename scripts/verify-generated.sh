#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
source "$repo_root/toolchain.env"
temp_dir="$(mktemp -d)"
trap 'rm -rf -- "$temp_dir"' EXIT
mkdir -p "$temp_dir/bin" "$temp_dir/gen"
GOBIN="$temp_dir/bin" go install "google.golang.org/protobuf/cmd/protoc-gen-go@$PROTOBUF_VERSION"
GOBIN="$temp_dir/bin" go install "google.golang.org/grpc/cmd/protoc-gen-go-grpc@$GRPC_PLUGIN_VERSION"
PATH="$temp_dir/bin:$PATH" protoc -I "$repo_root/api" --go_out="$temp_dir/gen" --go_opt=paths=source_relative --go-grpc_out="$temp_dir/gen" --go-grpc_opt=paths=source_relative "$repo_root/api/replicas/v1/replicas.proto"
diff -u "$repo_root/gen/replicas/v1/replicas.pb.go" "$temp_dir/gen/replicas/v1/replicas.pb.go"
diff -u "$repo_root/gen/replicas/v1/replicas_grpc.pb.go" "$temp_dir/gen/replicas/v1/replicas_grpc.pb.go"
