#!/usr/bin/env bash
# Собирает openapi.yaml со всеми $ref в один файл bundled-openapi.yaml.
set -euo pipefail

# спека резолвится относительно каталога скрипта, а не текущего каталога вызывающего
cd "$(dirname "$0")"

docker run --rm -v "$PWD:/spec" redocly/cli bundle openapi.yaml --output bundled-openapi.yaml
