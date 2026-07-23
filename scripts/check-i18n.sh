#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
locales="zh-CN ja ko es pt-BR de fr"
catalog_dir="$root/internal/ui/web/locales"

tmp_dir=$(mktemp -d "${TMPDIR:-/tmp}/stateseal-i18n.XXXXXX")
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

if ! command -v jq >/dev/null 2>&1; then
	echo "jq is required to validate UI locale catalogs" >&2
	exit 1
fi

jq -r 'keys[]' "$catalog_dir/en.json" | sort >"$tmp_dir/en.keys"
for locale in $locales; do
	catalog="$catalog_dir/$locale.json"
	test -f "$catalog" || { echo "missing UI catalog: $catalog" >&2; exit 1; }
	jq -e . "$catalog" >/dev/null
	jq -r 'keys[]' "$catalog" | sort >"$tmp_dir/$locale.keys"
	if ! diff -u "$tmp_dir/en.keys" "$tmp_dir/$locale.keys"; then
		echo "UI catalog keys differ for $locale" >&2
		exit 1
	fi
	for page in index.md getting-started.md runs-console.md; do
		test -f "$root/docs/$locale/$page" || {
			echo "missing core documentation: docs/$locale/$page" >&2
			exit 1
		}
	done
done

for readme in README.zh-CN.md README.ja.md README.ko.md README.es.md README.pt-BR.md README.de.md README.fr.md; do
	test -f "$root/$readme" || { echo "missing localized README: $readme" >&2; exit 1; }
done

english_example='Fix duplicate callbacks that cause duplicate charges while preserving API compatibility'
grep -Fq "seal run \"$english_example\"" "$root/README.md" || {
	echo "README.md is missing the canonical English seal run example" >&2
	exit 1
}
if grep -Fq 'seal run "修复' "$root/README.md"; then
	echo "README.md contains a Chinese seal run example" >&2
	exit 1
fi

for asset in stateseal-trust-pipeline.svg stateseal-runs-console.svg; do
	test -f "$root/docs/assets/$asset" || {
		echo "missing shared documentation visual: docs/assets/$asset" >&2
		exit 1
	}
	for readme in README.md README.zh-CN.md README.ja.md README.ko.md README.es.md README.pt-BR.md README.de.md README.fr.md; do
		grep -Fq "docs/assets/$asset" "$root/$readme" || {
			echo "$readme does not reference shared visual: $asset" >&2
			exit 1
		}
	done
done

base_sha=${BASE_SHA:-}
if [ -n "$base_sha" ] && git -C "$root" cat-file -e "$base_sha^{commit}" 2>/dev/null; then
	git -C "$root" diff --name-only "$base_sha"...HEAD >"$tmp_dir/changed"
	require_changed() {
		source=$1
		shift
		if ! grep -Fqx "$source" "$tmp_dir/changed"; then
			return
		fi
		for translation in "$@"; do
			if ! grep -Fqx "$translation" "$tmp_dir/changed"; then
				echo "$source changed without synchronized translation: $translation" >&2
				exit 1
			fi
		done
	}
	require_changed README.md \
		README.zh-CN.md README.ja.md README.ko.md README.es.md README.pt-BR.md README.de.md README.fr.md
	for page in index.md getting-started.md runs-console.md; do
		translations=""
		for locale in $locales; do
			translations="$translations docs/$locale/$page"
		done
		# Word splitting is intentional: the paths contain no whitespace.
		# shellcheck disable=SC2086
		require_changed "docs/$page" $translations
	done
fi

echo "StateSeal i18n contract passed: 8 UI catalogs and 8 core documentation locales."
