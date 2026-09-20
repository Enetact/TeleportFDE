# Generated wire bindings

Run `make prepare` (or `make generate`) on a networked development machine with
Go 1.25+ and protoc installed. The output belongs in:

- `gen/replicas/v1/replicas.pb.go`
- `gen/replicas/v1/replicas_grpc.pb.go`

The canonical input is `api/replicas/v1/replicas.proto`. Generated Go source was
not fabricated in an environment without the compiler/plugins. The Makefile pins
the Go plugin versions. Commit the real generated outputs alongside go.sum before
using the CI lock/regeneration check.
