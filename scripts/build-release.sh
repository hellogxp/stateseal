#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
version=${VERSION:-}
if [ -z "$version" ]; then
	echo "VERSION is required (for example, v0.1.0-alpha.1)" >&2
	exit 2
fi
if ! printf '%s\n' "$version" | grep -Eq '^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$'; then
	echo "VERSION must be a valid v-prefixed semantic version" >&2
	exit 2
fi

commit=${COMMIT:-$(git -C "$root" rev-parse HEAD)}
if ! printf '%s\n' "$commit" | grep -Eq '^[0-9a-f]{40}$'; then
	echo "COMMIT must be a full Git object ID" >&2
	exit 2
fi
build_date=${BUILD_DATE:-$(git -C "$root" show -s --format=%cI "$commit")}
dist=${DIST_DIR:-$root/dist}
targets=${TARGETS:-"darwin/amd64 darwin/arm64 linux/amd64 linux/arm64"}
module=github.com/hellogxp/stateseal/internal/buildinfo
ldflags="-s -w -X $module.Version=$version -X $module.Commit=$commit -X $module.Date=$build_date"

mkdir -p "$dist"
rm -f "$dist"/stateseal_*.tar.gz "$dist"/checksums.txt "$dist"/install.sh

for target in $targets; do
	goos=${target%/*}
	goarch=${target#*/}
	case "$goos/$goarch" in
		darwin/amd64|darwin/arm64|linux/amd64|linux/arm64) ;;
		*) echo "unsupported release target: $target" >&2; exit 2 ;;
	esac

	name="stateseal_${version#v}_${goos}_${goarch}"
	stage=$(mktemp -d "${TMPDIR:-/tmp}/stateseal-release.XXXXXX")
	mkdir -p "$stage/$name"
	CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -C "$root" -trimpath -ldflags "$ldflags" -o "$stage/$name/seal" ./cmd/seal
	cp "$root/README.md" "$root/LICENSE" "$root/SECURITY.md" "$stage/$name/"
	COPYFILE_DISABLE=1 tar -C "$stage" -czf "$dist/$name.tar.gz" "$name"
	rm -rf "$stage"
done

(
	cd "$dist"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum stateseal_*.tar.gz > checksums.txt
	else
		shasum -a 256 stateseal_*.tar.gz > checksums.txt
	fi
)

sed "s/STATESEAL_DEFAULT_VERSION:-latest/STATESEAL_DEFAULT_VERSION:-$version/" \
	"$root/scripts/install.sh" >"$dist/install.sh"
chmod 0755 "$dist/install.sh"
sh -n "$dist/install.sh"

printf 'Built StateSeal %s for %s\n' "$version" "$targets"
