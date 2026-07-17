#!/bin/sh
set -eu

repository=${STATESEAL_REPOSITORY:-hellogxp/stateseal}
version=${STATESEAL_VERSION:-latest}
install_dir=${STATESEAL_INSTALL_DIR:-$HOME/.local/bin}

usage() {
	cat <<'EOF'
Install a StateSeal release binary and verify its published checksum.

Usage: install.sh [--version VERSION] [--install-dir DIRECTORY]

Environment:
  STATESEAL_VERSION       Release tag or version (default: latest)
  STATESEAL_INSTALL_DIR   Destination directory (default: ~/.local/bin)
  STATESEAL_REPOSITORY    GitHub owner/repository (default: hellogxp/stateseal)
EOF
}

while [ "$#" -gt 0 ]; do
	case "$1" in
		--version) version=${2:?missing value for --version}; shift 2 ;;
		--install-dir) install_dir=${2:?missing value for --install-dir}; shift 2 ;;
		-h|--help) usage; exit 0 ;;
		*) echo "unknown argument: $1" >&2; usage >&2; exit 2 ;;
	esac
done

command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 1; }

if [ "$version" = latest ]; then
	latest_url=$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$repository/releases/latest")
	version=${latest_url##*/}
fi
case "$version" in v*) ;; *) version="v$version" ;; esac
if ! printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'; then
	echo "invalid release version: $version" >&2
	exit 2
fi

case $(uname -s) in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) echo "unsupported operating system: $(uname -s)" >&2; exit 1 ;;
esac
case $(uname -m) in
	x86_64|amd64) arch=amd64 ;;
	aarch64|arm64) arch=arm64 ;;
	*) echo "unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

name="stateseal_${version#v}_${os}_${arch}"
base_url="https://github.com/$repository/releases/download/$version"
tmp=$(mktemp -d "${TMPDIR:-/tmp}/stateseal-install.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

curl -fsSL "$base_url/$name.tar.gz" -o "$tmp/$name.tar.gz"
curl -fsSL "$base_url/checksums.txt" -o "$tmp/checksums.txt"
expected=$(awk -v file="$name.tar.gz" '$2 == file || $2 == "*" file { print $1 }' "$tmp/checksums.txt")
if [ -z "$expected" ]; then
	echo "release checksum is missing for $name.tar.gz" >&2
	exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$tmp/$name.tar.gz" | awk '{print $1}')
else
	actual=$(shasum -a 256 "$tmp/$name.tar.gz" | awk '{print $1}')
fi
if [ "$actual" != "$expected" ]; then
	echo "checksum verification failed for $name.tar.gz" >&2
	exit 1
fi

tar -C "$tmp" -xzf "$tmp/$name.tar.gz"
mkdir -p "$install_dir"
install -m 0755 "$tmp/$name/seal" "$install_dir/seal"
printf 'Installed StateSeal %s to %s/seal\n' "$version" "$install_dir"
"$install_dir/seal" version
