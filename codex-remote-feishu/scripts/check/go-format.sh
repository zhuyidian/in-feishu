#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

files=()
while IFS= read -r -d '' file; do
  files+=("${file}")
done < <(find cmd internal testkit -name '*.go' -print0 | sort -z)

if [[ ${#files[@]} -eq 0 ]]; then
  exit 0
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

normalized_files=()
for file in "${files[@]}"; do
  normalized_file="${tmp_dir}/${file}"
  mkdir -p "$(dirname "${normalized_file}")"
  # gofmt considers CRLF unformatted; preserve the user's Windows checkout.
  sed 's/\r$//' "${file}" > "${normalized_file}"
  normalized_files+=("${normalized_file}")
done

output="$(
  printf '%s\0' "${normalized_files[@]}" |
    xargs -0 -n 200 gofmt -l |
    while IFS= read -r formatted_file; do
      printf '%s\n' "${formatted_file#"${tmp_dir}/"}"
    done
)"
if [[ -z "${output}" ]]; then
  exit 0
fi

echo "${output}" >&2
echo "Run make fmt to format remaining Go files before continuing." >&2
exit 1
