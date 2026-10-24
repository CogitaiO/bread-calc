#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:?Использование: ./build.sh v1.0.0}"
export CGO_ENABLED=0

rm -rf dist && mkdir -p dist

build() {
  local os=$1 arch=$2 ext=$3
  local name="breadcalc-${VERSION}-${os}-${arch}"
  local dir="dist/${name}"
  mkdir -p "$dir"
  GOOS=$os GOARCH=$arch go build -trimpath -ldflags "-s -w" -o "${dir}/breadcalc${ext}" .
  cp README.md "$dir/"
  if [ "$os" = "windows" ]; then
    (cd dist && zip -qr "${name}.zip" "$name")
  else
    (cd dist && tar -czf "${name}.tar.gz" "$name")
  fi
  rm -rf "$dir"
}

build windows amd64 .exe
build linux   amd64 ""
build linux   arm64 ""
build darwin  amd64 ""   # Intel Mac
build darwin  arm64 ""   # Apple Silicon

(cd dist && sha256sum * > checksums.txt)
ls -lh dist