# Generated Go bindings

`replicas/v1/replicas.pb.go` and `replicas/v1/replicas_grpc.pb.go` are generated
from `api/replicas/v1/replicas.proto` with the versions in `toolchain.env`.
Keep these files and `go.sum` in source control. Do not edit generated Go by hand.

Both outputs are supplied in this checkout. For a normal build, run the root
`quality` and `vuln` targets after [toolchain setup](../docs/DEVELOPMENT.md).
The [development visual guide](../docs/visuals/delivery.html) shows how generation
verification fits into the build.

After intentionally changing the schema or generator versions, start in the
repository root (not `gen/`):

```bash
repo_root="$(pwd)"
bash "$repo_root/scripts/dev.sh" prepare
bash "$repo_root/scripts/dev.sh" quality vuln
```

`verify-generated` regenerates into a temporary directory and compares it with
these files. It does not silently repair the working tree.
The common formatter includes generated `.go` files in both `format` and
`format-check`. Review and commit the real generated outputs and module-file
changes together; do not replace them with handwritten stubs.
