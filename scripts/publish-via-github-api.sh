#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 || $# -gt 3 ]]; then
  echo "usage: GH_TOKEN=... $0 <owner/repo> <branch> [commit-ish]" >&2
  exit 2
fi
if [[ -z "${GH_TOKEN:-}" ]]; then
  echo "GH_TOKEN is required" >&2
  exit 2
fi

REPOSITORY=$1
BRANCH=$2
SOURCE_COMMIT=${3:-HEAD}
API="https://api.github.com/repos/$REPOSITORY"
WORK=$(mktemp -d "${TMPDIR:-/tmp}/stateseal-publish.XXXXXX")
trap 'rm -r "$WORK"' EXIT

request() {
  local method=$1
  local url=$2
  local data=${3:-}
  if [[ -n "$data" ]]; then
    curl -fsS --http1.1 --retry 5 --retry-all-errors --connect-timeout 10 -X "$method" \
      -H "Authorization: Bearer $GH_TOKEN" \
      -H 'Accept: application/vnd.github+json' \
      -H 'X-GitHub-Api-Version: 2022-11-28' \
      "$url" -d "@$data"
  else
    curl -fsS --http1.1 --retry 5 --retry-all-errors --connect-timeout 10 -X "$method" \
      -H "Authorization: Bearer $GH_TOKEN" \
      -H 'Accept: application/vnd.github+json' \
      -H 'X-GitHub-Api-Version: 2022-11-28' \
      "$url"
  fi
}

PARENT=$(request GET "$API/git/ref/heads/$BRANCH" | jq -er '.object.sha')
printf '[]\n' > "$WORK/entries.json"

while IFS= read -r file; do
  git show "$SOURCE_COMMIT:$file" | base64 | tr -d '\n' > "$WORK/content.b64"
  jq -n --rawfile content "$WORK/content.b64" '{content:$content,encoding:"base64"}' > "$WORK/blob-request.json"
  BLOB=$(request POST "$API/git/blobs" "$WORK/blob-request.json" | jq -er '.sha')
  MODE=$(git ls-tree "$SOURCE_COMMIT" -- "$file" | awk '{print $1}')
  jq --arg path "$file" --arg mode "$MODE" --arg sha "$BLOB" \
    '. + [{path:$path,mode:$mode,type:"blob",sha:$sha}]' \
    "$WORK/entries.json" > "$WORK/entries.next.json"
  mv "$WORK/entries.next.json" "$WORK/entries.json"
done < <(git ls-tree -r --name-only "$SOURCE_COMMIT")

# Build a complete tree from the source commit. Supplying base_tree here would
# preserve remote paths that were deleted locally, producing an invalid mirror.
jq -n --slurpfile tree "$WORK/entries.json" \
  '{tree:$tree[0]}' > "$WORK/tree-request.json"
TREE=$(request POST "$API/git/trees" "$WORK/tree-request.json" | jq -er '.sha')
MESSAGE=$(git log -1 --format=%B "$SOURCE_COMMIT")
jq -n --arg message "$MESSAGE" --arg tree "$TREE" --arg parent "$PARENT" \
  '{message:$message,tree:$tree,parents:[$parent]}' > "$WORK/commit-request.json"
COMMIT=$(request POST "$API/git/commits" "$WORK/commit-request.json" | jq -er '.sha')
jq -n --arg sha "$COMMIT" '{sha:$sha,force:false}' > "$WORK/ref-request.json"
request PATCH "$API/git/refs/heads/$BRANCH" "$WORK/ref-request.json" | jq '{ref,sha:.object.sha}'
