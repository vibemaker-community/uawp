#!/bin/sh
set -eu

version=""
destination="${HOME}/.local/bin"
force=0

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version) version=${2-}; shift 2 ;;
    --dest) destination=${2-}; shift 2 ;;
    --force) force=1; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [ -z "$version" ]; then
  echo "--version is required" >&2
  exit 2
fi
version=${version#v}

platform="$(uname -s):$(uname -m)"
if [ "${UAWP_INSTALL_TESTING:-}" = "1" ] && [ -n "${UAWP_INSTALL_TEST_PLATFORM:-}" ]; then
  platform=$UAWP_INSTALL_TEST_PLATFORM
fi
case "$platform" in
  Darwin:arm64) target=darwin_arm64 ;;
  Darwin:x86_64) target=darwin_amd64 ;;
  Linux:x86_64|Linux:amd64) target=linux_amd64 ;;
  Linux:aarch64|Linux:arm64) target=linux_arm64 ;;
  *) echo "unsupported platform: $platform" >&2; exit 1 ;;
esac

asset="uawp_${version}_${target}.tar.gz"
checksums="uawp_${version}_checksums.txt"
base_url="https://github.com/vibemaker-community/uawp/releases/download/v${version}"
if [ -n "${UAWP_INSTALL_BASE_URL:-}" ]; then
  if [ "${UAWP_INSTALL_TESTING:-}" != "1" ]; then
    echo "UAWP_INSTALL_BASE_URL is available only when UAWP_INSTALL_TESTING=1" >&2
    exit 2
  fi
  base_url=${UAWP_INSTALL_BASE_URL%/}
fi
case "$base_url" in
  https://*) ;;
  http://*) [ "${UAWP_INSTALL_TESTING:-}" = "1" ] || { echo "HTTPS is required" >&2; exit 1; } ;;
  *) echo "invalid release URL" >&2; exit 1 ;;
esac

final_path="${destination%/}/uawp"
if { [ -e "$final_path" ] || [ -L "$final_path" ]; } && [ "$force" -ne 1 ]; then
  echo "$final_path already exists; use --force to replace it" >&2
  exit 1
fi

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/uawp-install.XXXXXX")
staging_path=""
cleanup() {
  if [ -n "$staging_path" ] && [ -e "$staging_path" ]; then rm -f "$staging_path"; fi
  rm -rf "$work_dir"
}
trap cleanup EXIT HUP INT TERM

download() {
  if [ "${UAWP_INSTALL_TESTING:-}" = "1" ]; then
    curl -fsSL --tlsv1.2 -o "$1" "$2"
  else
    curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 -o "$1" "$2"
  fi
}
download "$work_dir/$checksums" "$base_url/$checksums"
download "$work_dir/$asset" "$base_url/$asset"

matching=$(awk -v name="$asset" '$2 == name { print $1 }' "$work_dir/$checksums")
line_count=$(printf '%s\n' "$matching" | awk 'NF { count++ } END { print count+0 }')
if [ "$line_count" -ne 1 ]; then
  echo "checksum file must contain exactly one entry for $asset" >&2
  exit 1
fi
expected_hash=$(printf '%s\n' "$matching" | awk 'NF { print $1 }')
if [ "${#expected_hash}" -ne 64 ]; then
  echo "invalid SHA-256 value for $asset" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  actual_hash=$(sha256sum "$work_dir/$asset" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  actual_hash=$(LC_ALL=C LANG=C shasum -a 256 "$work_dir/$asset" | awk '{print $1}')
else
  echo "sha256sum or shasum is required" >&2
  exit 1
fi
if [ "$actual_hash" != "$expected_hash" ]; then
  echo "SHA-256 verification failed for $asset" >&2
  exit 1
fi

expected_entries='LICENSE
NOTICE
README.md
THIRD_PARTY_NOTICES.md
TRADEMARKS.md
release-metadata.json
uawp'
actual_entries=$(LC_ALL=C LANG=C tar -tzf "$work_dir/$asset" | LC_ALL=C sort)
if [ "$actual_entries" != "$expected_entries" ]; then
  echo "archive contains missing, duplicate, or unsafe entries" >&2
  exit 1
fi
if LC_ALL=C LANG=C tar -tvzf "$work_dir/$asset" | awk '$1 !~ /^-/ { bad=1 } END { exit bad }'; then :; else
  echo "archive contains a non-regular entry" >&2
  exit 1
fi

mkdir "$work_dir/extract"
LC_ALL=C LANG=C tar -xzf "$work_dir/$asset" -C "$work_dir/extract" uawp
chmod 0755 "$work_dir/extract/uawp"
version_output=$("$work_dir/extract/uawp" version --format json) || {
  echo "downloaded uawp executable failed its version check" >&2
  exit 1
}
case "$version_output" in
  *\"version\":\"$version\"*) ;;
  *) echo "downloaded uawp executable reports the wrong version" >&2; exit 1 ;;
esac
mkdir -p "$destination"
staging_path="${destination%/}/.uawp-install.$$"
cp "$work_dir/extract/uawp" "$staging_path"
chmod 0755 "$staging_path"
if [ "$force" -eq 1 ]; then
  mv -f "$staging_path" "$final_path"
else
  if ! ln "$staging_path" "$final_path" 2>/dev/null; then
    echo "$final_path appeared during installation; use --force to replace it" >&2
    exit 1
  fi
  rm -f "$staging_path"
fi
staging_path=""
echo "Installed UAWP $version to $final_path"
echo "PATH was not changed. Add the destination yourself if needed."
