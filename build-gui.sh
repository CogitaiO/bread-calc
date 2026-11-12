#!/usr/bin/env bash
set -euo pipefail

wayland=false
if [ "${1:-}" = "--wayland" ]; then
  wayland=true
  shift
  if [ "$(go env GOOS)" != "linux" ]; then
    echo "Параметр --wayland поддерживается только в Linux." >&2
    exit 1
  fi
fi
output="${1:?Использование: ./build-gui.sh [--wayland] ПУТЬ_К_ИСПОЛНЯЕМОМУ_ФАЙЛУ}"
if [ "$#" -ne 1 ]; then
  echo "Ожидается один путь к выходному файлу" >&2
  exit 1
fi
if [ -e "$output" ] || [ -L "$output" ]; then
  echo "Файл уже существует, выберите другой путь: $output" >&2
  exit 1
fi
if [ ! -d "$(dirname "$output")" ]; then
  echo "Каталог для сборки не существует: $(dirname "$output")" >&2
  exit 1
fi
if [ "$(go env GOOS)" != "$(go env GOHOSTOS)" ] || [ "$(go env GOARCH)" != "$(go env GOHOSTARCH)" ]; then
  echo "Скрипт поддерживает только нативную сборку; для кросс-компиляции Fyne нужен соответствующий C-компилятор." >&2
  exit 1
fi
if [ "$(go env CGO_ENABLED)" != "1" ]; then
  echo "Для GUI необходим CGO_ENABLED=1 и установленный C-компилятор." >&2
  exit 1
fi

if [ "$wayland" = true ]; then
  go build -tags wayland -trimpath -ldflags "-s -w" -o "$output" ./cmd/breadcalc-gui
else
  go build -trimpath -ldflags "-s -w" -o "$output" ./cmd/breadcalc-gui
fi
