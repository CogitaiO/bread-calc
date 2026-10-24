#!/usr/bin/env bash
set -euo pipefail

VERSION="${1:?Использование: ./build.sh v1.0.1}"
export CGO_ENABLED=0

rm -rf dist && mkdir -p dist

build() {
  local os=$1 arch=$2 ext=$3
  local label=$arch
  [ "$arch" = "amd64" ] && label="x86_64"   # понятное имя для пользователей
  local name="breadcalc-${VERSION}-${os}-${label}"
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

build windows amd64 .exe   # обычный ПК с Windows
build windows arm64 .exe   # Windows на ARM (Snapdragon)
build linux   amd64 ""     # обычный ПК с Linux
build linux   arm64 ""     # Raspberry Pi 4/5, ARM-ноутбуки
build darwin  arm64 ""     # Mac на M1 и новее
build darwin  amd64 ""     # Mac на Intel

(cd dist && sha256sum * > checksums.txt)
ls -lh dist