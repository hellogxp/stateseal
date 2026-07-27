#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
installer=${1:-$root/dist/install.sh}
fixtures=${STATESEAL_INSTALL_FIXTURES:-$root/dist}
expected_version=${EXPECTED_VERSION:-}
expected_commit=${EXPECTED_COMMIT:-}

fixtures=$(CDPATH= cd -- "$fixtures" && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/stateseal-install-test.XXXXXX")
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/bin" "$tmp/install"

cat >"$tmp/bin/curl" <<'EOF'
#!/bin/sh
set -eu

output=
url=
while [ "$#" -gt 0 ]; do
	case "$1" in
		-o) output=${2:?missing curl output}; shift 2 ;;
		-H|--proto|-w) shift 2 ;;
		--tlsv1.2|-fsSL) shift ;;
		https://*) url=$1; shift ;;
		*) shift ;;
	esac
done

test -n "$output"
test -n "$url"
cp "$STATESEAL_INSTALL_FIXTURES/${url##*/}" "$output"
EOF
chmod 0755 "$tmp/bin/curl"

PATH="$tmp/bin:$PATH" \
	STATESEAL_INSTALL_FIXTURES="$fixtures" \
	STATESEAL_INSTALL_DIR="$tmp/install" \
	sh "$installer" >"$tmp/install.log"

"$tmp/install/seal" version --json >"$tmp/version.json"
if [ -n "$expected_version" ]; then
	grep -Fq "\"version\":\"$expected_version\"" "$tmp/version.json"
fi
if [ -n "$expected_commit" ]; then
	grep -Fq "\"commit\":\"$expected_commit\"" "$tmp/version.json"
fi
printf 'Installer smoke test passed: %s\n' "$(cat "$tmp/version.json")"
