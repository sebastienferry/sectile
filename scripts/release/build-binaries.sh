#!/bin/sh
#
# Cross-compile the server and the agent for every supported OS/arch.
#
# Usage: scripts/release/build-binaries.sh <version> <commit> <outdir>
#
# Both release streams call it, the GitLab tag pipeline and the GitHub release
# workflow, so the two cannot drift apart on targets, flags or file names. The
# file names carry the product name so the binaries can be installed as-is
# under the canonical command names. Run it from the repository root, with the
# interface already built into internal/webui/dist: the server embeds it.
set -eu

if [ $# -ne 3 ]; then
  echo "usage: $0 <version> <commit> <outdir>" >&2
  exit 2
fi
version=$1
commit=$2
outdir=$3

test -f internal/webui/dist/index.html || { echo "internal/webui/dist is empty, the server would embed no interface" >&2; exit 1; }
mkdir -p "$outdir"

# The version is what every binary reports for --version and on /api/version.
# The caller passes the tag rather than letting it be derived, so the string a
# user quotes in a bug report is a ref this repository can check out.
version_pkg=tasks/internal/version
version_flags="-X ${version_pkg}.Version=${version} -X ${version_pkg}.Commit=${commit} -X ${version_pkg}.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"

for target in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64; do
  os=${target%/*}; arch=${target#*/}
  for component in server agent; do
    out=${outdir}/sectile-${component}-${os}-${arch}
    if [ "$os" = "windows" ]; then out=${out}.exe; fi
    echo "  ${component} ${os}/${arch}"
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "-s -w ${version_flags}" -o "$out" "./cmd/${component}"
  done
done
