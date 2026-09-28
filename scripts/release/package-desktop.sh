#!/bin/sh
#
# Package Sectile Desktop for one target and archive it for download.
#
# Usage: scripts/release/package-desktop.sh <tag> <os> <arch> <agent> <outdir>
#
#   <os> <arch>  the Go tokens the agent was built for: darwin/arm64,
#                darwin/amd64, linux/amd64 or windows/amd64
#   <agent>      that agent, sectile-agent-<os>-<arch>[.exe]
#   <outdir>     where sectile-desktop-<os>-<arch>.<zip|tar.gz> is written
#
# The desktop app must already be built (npm ci and npm run build in desktop/).
# The archive holds the one directory @electron/packager writes,
# Sectile-<platform>-<arch>/, with the agent at the place the packaged app
# resolves it from. macOS and Windows get a zip, stored with -y so the framework
# symlinks inside Sectile.app stay links; Linux gets a tarball, which keeps the
# executable bits without the user running chmod.
set -eu

if [ $# -ne 5 ]; then
  echo "usage: $0 <tag> <os> <arch> <agent> <outdir>" >&2
  exit 2
fi
tag=$1
os=$2
arch=$3
agent=$4
outdir=$5

case "$os" in
  darwin | linux) platform=$os ;;
  windows) platform=win32 ;;
  *) echo "unsupported os ${os}" >&2; exit 2 ;;
esac
case "$arch" in
  arm64) electron_arch=arm64 ;;
  amd64) electron_arch=x64 ;;
  *) echo "unsupported arch ${arch}" >&2; exit 2 ;;
esac
case "$platform" in
  darwin) archive=sectile-desktop-${os}-${arch}.zip; bundled=Sectile.app/Contents/Resources/sectile-agent ;;
  linux) archive=sectile-desktop-${os}-${arch}.tar.gz; bundled=resources/sectile-agent ;;
  win32) archive=sectile-desktop-${os}-${arch}.zip; bundled=resources/sectile-agent.exe ;;
esac

test -f "$agent" || { echo "agent not found: ${agent}" >&2; exit 1; }
root=$(cd "$(dirname "$0")/../.." && pwd)
test -f "${root}/desktop/dist/index.html" || { echo "desktop/dist is empty, build the desktop app first" >&2; exit 1; }
agent=$(cd "$(dirname "$agent")" && pwd)/$(basename "$agent")
mkdir -p "$outdir"
outdir=$(cd "$outdir" && pwd)

# Packager writes into a directory of its own, removed once archived: four
# extracted Electron apps would otherwise fill the runner's disk.
work=${outdir}/.package-${os}-${arch}
rm -rf "$work"
trap 'rm -rf "$work"' EXIT
(cd "${root}/desktop" && node electron/package.cjs --platform "$platform" --arch "$electron_arch" --agent "$agent" --out "$work")

app=Sectile-${platform}-${electron_arch}
test -f "${work}/${app}/${bundled}" || { echo "the package has no agent at ${app}/${bundled}" >&2; exit 1; }

# The agent was stamped with the tag, and the app it ships in reports the same
# release. It can only be run here for the runner's own platform, which is
# where the Linux archive is built.
if [ "$platform" = linux ] && [ "$(uname -s)" = Linux ] && [ "$(uname -m)" = x86_64 ] && [ "$electron_arch" = x64 ]; then
  "${work}/${app}/${bundled}" --version | grep -F "$tag" || { echo "the bundled agent does not report ${tag}" >&2; exit 1; }
elif [ "$platform" = linux ]; then
  echo "not a linux/amd64 host, the bundled agent's --version is not checked"
fi

rm -f "${outdir}/${archive}"
case "$archive" in
  *.zip) (cd "$work" && zip -qry "${outdir}/${archive}" "$app") ;;
  *.tar.gz) tar -czf "${outdir}/${archive}" -C "$work" "$app" ;;
esac
echo "${outdir}/${archive}"
