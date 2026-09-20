# Generated Go bindings

`replicas/v1/replicas.pb.go` and `replicas/v1/replicas_grpc.pb.go` are generated
from `api/replicas/v1/replicas.proto` with the versions in `toolchain.env`.
Keep these files and `go.sum` in source control. Do not edit generated Go by hand.

After intentionally changing the schema or generator versions:

```bash
source scripts/dev-env.sh
make prepare
make verify-generated test
```

`verify-generated` regenerates into a temporary directory and compares it with
these files. It does not silently repair the working tree.
