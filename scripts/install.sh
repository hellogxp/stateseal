#!/bin/sh
set -eu

repository=${STATESEAL_REPOSITORY:-hellogxp/stateseal}
version=${STATESEAL_VERSION:-${STATESEAL_DEFAULT_VERSION:-latest}}
install_dir=${STATESEAL_INSTALL_DIR:-$HOME/.local/bin}
github_token=${STATESEAL_GITHUB_TOKEN:-}

usage() {
	cat <<'EOF'
Install a StateSeal release binary and verify its published checksum.

Usage: install.sh [--version VERSION] [--install-dir DIRECTORY]

Environment:
  STATESEAL_VERSION       Release tag or version (default: latest)
  STATESEAL_INSTALL_DIR   Destination directory (default: ~/.local/bin)
  STATESEAL_REPOSITORY    GitHub owner/repository (default: hellogxp/stateseal)
  STATESEAL_GITHUB_TOKEN  Token used only when downloading from a private repository
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

for command in curl tar install awk grep; do
	command -v "$command" >/dev/null 2>&1 || { echo "$command is required" >&2; exit 1; }
done

fetch() {
	if [ -n "$github_token" ]; then
		curl --proto '=https' --tlsv1.2 -fsSL \
			-H "Authorization: Bearer $github_token" \
			-H "X-GitHub-Api-Version: 2022-11-28" "$@"
	else
		curl --proto '=https' --tlsv1.2 -fsSL "$@"
	fi
}

if [ "$version" = latest ]; then
	if [ -n "$github_token" ]; then
		latest_url=$(fetch -o /dev/null -w '%{url_effective}' "https://github.com/$repository/releases/latest")
	else
		latest_url=$(curl --proto '=https' --tlsv1.2 -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$repository/releases/latest")
	fi
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
install_tmp=
trap 'rm -rf "$tmp"; if [ -n "$install_tmp" ]; then rm -f "$install_tmp"; fi' EXIT HUP INT TERM

fetch "$base_url/$name.tar.gz" -o "$tmp/$name.tar.gz"
fetch "$base_url/checksums.txt" -o "$tmp/checksums.txt"
expected=$(awk -v file="$name.tar.gz" '$2 == file || $2 == "*" file { print $1 }' "$tmp/checksums.txt")
if ! printf '%s\n' "$expected" | grep -Eq '^[0-9a-fA-F]{64}$'; then
	echo "release checksum is missing for $name.tar.gz" >&2
	exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$tmp/$name.tar.gz" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$tmp/$name.tar.gz" | awk '{print $1}')
else
	echo "sha256sum or shasum is required" >&2
	exit 1
fi
if [ "$actual" != "$expected" ]; then
	echo "checksum verification failed for $name.tar.gz" >&2
	exit 1
fi

tar -C "$tmp" -xzf "$tmp/$name.tar.gz"
mkdir -p "$install_dir"
install_tmp="$install_dir/.seal.install.$$"
install -m 0755 "$tmp/$name/seal" "$install_tmp"
mv -f "$install_tmp" "$install_dir/seal"
install_tmp=
printf 'Installed StateSeal %s to %s/seal\n' "$version" "$install_dir"
"$install_dir/seal" version
