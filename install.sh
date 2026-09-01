#!/bin/sh
set -eu

repository="mikker/wt"
install_dir="${WT_INSTALL_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
  Darwin) platform="darwin" ;;
  Linux) platform="linux" ;;
  *) echo "wt: unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  arm64|aarch64) arch="arm64" ;;
  x86_64|amd64) arch="amd64" ;;
  *) echo "wt: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

for command in curl tar; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "wt: $command is required" >&2
    exit 1
  }
done

if [ -n "${WT_VERSION:-}" ]; then
  version="$WT_VERSION"
else
  latest_url="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$repository/releases/latest")"
  version="${latest_url##*/}"
fi

archive="wt_${version}_${platform}_${arch}.tar.gz"
release_url="https://github.com/$repository/releases/download/$version"

tmp_dir="$(mktemp -d 2>/dev/null || mktemp -d -t wt)"
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

echo "Downloading wt $version for $platform/$arch..."
curl -fsSL "$release_url/$archive" -o "$tmp_dir/$archive"
curl -fsSL "$release_url/checksums.txt" -o "$tmp_dir/checksums.txt"

expected="$(awk -v archive="$archive" '$2 == archive { print $1; exit }' "$tmp_dir/checksums.txt")"
if [ -z "$expected" ]; then
  echo "wt: checksum not found for $archive" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp_dir/$archive" | awk '{ print $1 }')"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "$tmp_dir/$archive" | awk '{ print $1 }')"
else
  echo "wt: sha256sum or shasum is required" >&2
  exit 1
fi

if [ "$actual" != "$expected" ]; then
  echo "wt: checksum verification failed" >&2
  exit 1
fi

tar -xzf "$tmp_dir/$archive" -C "$tmp_dir"
mkdir -p "$install_dir"
cp "$tmp_dir/wt" "$install_dir/.wt.$$"
chmod 755 "$install_dir/.wt.$$"
mv "$install_dir/.wt.$$" "$install_dir/wt"

echo "Installed $("$install_dir/wt" --version) to $install_dir/wt"
case ":${PATH:-}:" in
  *":$install_dir:"*) ;;
  *) echo "Add $install_dir to your PATH to run wt." ;;
esac
