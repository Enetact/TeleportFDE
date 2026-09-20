"""Check that dependency, image and generator pins agree before building."""
from pathlib import Path
import re

root = Path(__file__).resolve().parent.parent
pins = dict(line.split("=", 1) for line in (root / "toolchain.env").read_text().splitlines()
            if line and not line.startswith("#"))
module = (root / "go.mod").read_text()
def require(condition, message):
    if not condition:
        raise SystemExit(message)

for name in ("k8s.io/api", "k8s.io/apimachinery", "k8s.io/client-go"):
    version = re.search(r"\b" + re.escape(name) + r"\s+(v[\d.]+)", module).group(1)
    require(version.replace("v0.", "v1.", 1) == pins["KUBECTL_VERSION"],
            f"Align {name}, kubectl and the KIND Kubernetes version")
require(f'google.golang.org/protobuf {pins["PROTOBUF_VERSION"]}' in module,
        "protobuf runtime and protoc-gen-go versions differ")
require(f'toolchain go{pins["GO_VERSION"]}' in module, "Go toolchain selection differs")
require(f'golang:{pins["GO_VERSION"]}-' in pins["GO_IMAGE"], "Go builder image version differs")
require(f'kindest/node:{pins["KUBECTL_VERSION"]}@' in pins["KIND_NODE_IMAGE"], "KIND image differs")
require(f'ARG GO_IMAGE={pins["GO_IMAGE"]}' in (root / "Dockerfile").read_text(),
        "Dockerfile default and toolchain.env GO_IMAGE differ")
for key in ("GO_IMAGE", "KIND_NODE_IMAGE", "PAUSE_IMAGE"):
    require(re.search(r"@sha256:[a-f0-9]{64}$", pins[key]), f"Missing image digest: {key}")
for name in ("GO", "KIND", "KUBECTL", "HELM", "PROTOC"):
    for arch in ("AMD64", "ARM64"):
        require(re.fullmatch(r"[a-f0-9]{64}", pins[f"{name}_{arch}_SHA256"]),
                f"Invalid download checksum: {name}/{arch}")
print("Toolchain, generator and image mappings agree.")
