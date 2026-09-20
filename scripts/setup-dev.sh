#!/usr/bin/env bash
# Install pinned lab tools; the Docker daemon is reused when already installed.
set -euo pipefail
source "$(dirname -- "${BASH_SOURCE[0]}")/dev-env.sh"
[[ $(uname -s) == Linux ]] || { echo 'This installer supports Linux/WSL; see docs/DEVELOPMENT.md for macOS.' >&2; exit 1; }
case $(uname -m) in
  x86_64) arch=amd64; proto_arch=x86_64 ;;
  aarch64|arm64) arch=arm64; proto_arch=aarch_64 ;;
  *) echo 'Supported architectures: amd64 and arm64.' >&2; exit 1 ;;
esac
sudo_cmd=()
if [[ $EUID != 0 ]]; then sudo_cmd=(sudo); fi
"${sudo_cmd[@]}" apt-get update
"${sudo_cmd[@]}" env DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends build-essential ca-certificates curl git jq unzip python3
mkdir -p "$tool_home/bin" "$tool_home/downloads" "$tool_home/protoc-$PROTOC_VERSION"
download() {
  local url=$1 file=$2 checksum=$3
  if [[ ! -f $file ]] || ! printf '%s  %s\n' "$checksum" "$file" | sha256sum --check --status; then
    curl --fail --location --retry 3 --output "$file" "$url"
  fi
  printf '%s  %s\n' "$checksum" "$file" | sha256sum --check
}
checksum() { local key="${1}_${arch^^}_SHA256"; printf '%s' "${!key}"; }
go_archive="$tool_home/downloads/go-$GO_VERSION-$arch.tar.gz"
download "https://go.dev/dl/go$GO_VERSION.linux-$arch.tar.gz" "$go_archive" "$(checksum GO)"
if [[ ! -x $tool_home/go-$GO_VERSION/bin/go ]]; then
  mkdir -p "$tool_home/go-$GO_VERSION"
  tar -xzf "$go_archive" -C "$tool_home/go-$GO_VERSION" --strip-components=1
fi
download "https://github.com/kubernetes-sigs/kind/releases/download/$KIND_VERSION/kind-linux-$arch" "$tool_home/downloads/kind-$KIND_VERSION-$arch" "$(checksum KIND)"
install -m 0755 "$tool_home/downloads/kind-$KIND_VERSION-$arch" "$tool_home/bin/kind"
download "https://dl.k8s.io/release/$KUBECTL_VERSION/bin/linux/$arch/kubectl" "$tool_home/downloads/kubectl-$KUBECTL_VERSION-$arch" "$(checksum KUBECTL)"
install -m 0755 "$tool_home/downloads/kubectl-$KUBECTL_VERSION-$arch" "$tool_home/bin/kubectl"
helm_archive="$tool_home/downloads/helm-$HELM_VERSION-$arch.tar.gz"
download "https://get.helm.sh/helm-$HELM_VERSION-linux-$arch.tar.gz" "$helm_archive" "$(checksum HELM)"
tar -xzf "$helm_archive" -C "$tool_home/downloads" "linux-$arch/helm"
install -m 0755 "$tool_home/downloads/linux-$arch/helm" "$tool_home/bin/helm"
proto_archive="$tool_home/downloads/protoc-$PROTOC_VERSION-$arch.zip"
download "https://github.com/protocolbuffers/protobuf/releases/download/v$PROTOC_VERSION/protoc-$PROTOC_VERSION-linux-$proto_arch.zip" "$proto_archive" "$(checksum PROTOC)"
unzip -qo "$proto_archive" -d "$tool_home/protoc-$PROTOC_VERSION"
ln -sfn "$tool_home/protoc-$PROTOC_VERSION/bin/protoc" "$tool_home/bin/protoc"
GOBIN="$tool_home/bin" go install "github.com/rhysd/actionlint/cmd/actionlint@$ACTIONLINT_VERSION"
GOBIN="$tool_home/bin" go install "golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_VERSION"
if ! command -v docker >/dev/null; then
  source /etc/os-release
  [[ $ID == ubuntu ]] || { echo 'Install Docker Engine for your distribution, then rerun setup.' >&2; exit 1; }
  "${sudo_cmd[@]}" install -m 0755 -d /etc/apt/keyrings
  curl --fail --location https://download.docker.com/linux/ubuntu/gpg | "${sudo_cmd[@]}" tee /etc/apt/keyrings/docker.asc >/dev/null
  "${sudo_cmd[@]}" chmod a+r /etc/apt/keyrings/docker.asc
  printf 'deb [arch=%s signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu %s stable\n' "$(dpkg --print-architecture)" "$VERSION_CODENAME" | "${sudo_cmd[@]}" tee /etc/apt/sources.list.d/teleportfde-docker.list >/dev/null
  "${sudo_cmd[@]}" apt-get update
  "${sudo_cmd[@]}" env DEBIAN_FRONTEND=noninteractive apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  "${sudo_cmd[@]}" systemctl start docker
fi
go version
protoc --version
kind version
helm version --short
kubectl version --client
docker version
docker buildx version
printf '\nTools installed under %s. Run: bash scripts/dev.sh prepare\n' "$tool_home"
