#!/bin/sh
#
# Refuse a release whose desktop app would report another version than its tag.
#
# Usage: scripts/release/check-desktop-version.sh <tag>
#
# The packaged app reports desktop/package.json's version in its settings, and
# the release procedure bumps it before tagging. A tag cut without the bump
# would publish archives that lie about what they are, so both release streams
# run this before building anything. Run it from the repository root.
set -eu

if [ $# -ne 1 ]; then
  echo "usage: $0 <tag>" >&2
  exit 2
fi
tag=$1
version=$(node -p "require('./desktop/package.json').version")

if [ "v${version}" != "$tag" ]; then
  echo "desktop/package.json says ${version}, which does not match the tag ${tag}: bump the manifests before tagging (AGENTS.md, step 4)" >&2
  exit 1
fi
echo "desktop/package.json matches ${tag}"
