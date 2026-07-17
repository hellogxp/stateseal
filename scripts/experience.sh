#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
experience_root=${STATESEAL_EXPERIENCE_ROOT:-$(mktemp -d "${TMPDIR:-/tmp}/stateseal-experience.XXXXXX")}
repo=$experience_root/project
seal=${STATESEAL_BINARY:-$experience_root/seal}

if [ -e "$repo" ]; then
	echo "experience repository already exists: $repo" >&2
	exit 2
fi
mkdir -p "$repo"
cp -R "$root/examples/regression-recovery/." "$repo/"

if [ -z "${STATESEAL_BINARY:-}" ]; then
	go build -C "$root" -trimpath -o "$seal" ./cmd/seal
fi
if [ ! -x "$seal" ]; then
	echo "StateSeal binary is not executable: $seal" >&2
	exit 2
fi

git -C "$repo" init -q -b main
git -C "$repo" config user.name "StateSeal Experience"
git -C "$repo" config user.email "experience@stateseal.dev"
git -C "$repo" add .
git -C "$repo" commit -q -m "Initialize alarm deduplication fixture"

printf '\nStateSeal regression recovery\n\n'
(
	cd "$repo"
	"$seal" run -- "$repo/agent.sh" "$seal"
	printf '\nFinal state\n\n'
	"$seal" status
	printf '\nReliability timeline\n\n'
	"$seal" timeline
)

printf '\nExperience repository: %s\n' "$repo"
printf 'Inspect recovered code: cd %s && %s diff\n' "$repo" "$seal"
printf 'Apply it explicitly:    %s apply --branch recovered/alarm-deduplication\n' "$seal"
