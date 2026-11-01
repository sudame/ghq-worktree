#!/usr/bin/env bash
# release-bump.sh と next-version.sh を試す。next-version.sh は使い捨ての git リポジトリで試す。
set -euo pipefail

dir="$(cd "$(dirname "$0")" && pwd)"
failed=0
check() {
  local name="$1" got="$2" want="$3"
  if [ "$got" != "$want" ]; then
    echo "FAIL: $name: got $got, want $want" >&2
    failed=1
  fi
}

# 引数 1 つが PR 1 件のラベル（カンマ区切り）
bump() {
  if [ "$#" -gt 0 ]; then
    printf '%s\n' "$@"
  fi | bash "$dir/release-bump.sh"
}
check "PR なし" "$(bump)" skip
check "ラベルのない PR" "$(bump '')" patch
check "関係ないラベル" "$(bump 'bug,documentation')" patch
check "minor" "$(bump 'bug,release:minor' '')" minor
check "major" "$(bump 'release:major')" major
check "PR ごとにいちばん大きい桁" "$(bump 'release:minor' 'release:major' 'release:minor')" major
check "major のあとに patch" "$(bump 'release:major' '')" major
check "前方一致はしない" "$(bump 'release:major-ish')" patch
check "skip だけ" "$(bump 'release:skip' 'documentation,release:skip')" skip
check "skip の PR の桁は数えない" "$(bump 'release:skip,release:major' '')" patch
check "skip と minor" "$(bump 'release:skip' 'release:minor')" minor
check "最後の行に改行がない" "$(printf 'release:minor' | bash "$dir/release-bump.sh")" minor

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
cd "$work"
git init -q
git config user.email test@example.com
git config user.name test
git config tag.gpgSign false
git config commit.gpgSign false

next() {
  bash "$dir/next-version.sh" "$@"
}
commit() {
  git commit -q --allow-empty -m "$1"
}

commit first
check "タグなし patch" "$(next patch)" v0.1.0
check "タグなし minor" "$(next minor)" v0.1.0
check "タグなし major" "$(next major)" v0.1.0
check "タグなし skip" "$(next skip)" ""

git tag v0.1.0
check "HEAD にタグ patch" "$(next patch)" v0.1.0
check "HEAD にタグ major" "$(next major)" v0.1.0
check "HEAD にタグ skip" "$(next skip)" v0.1.0

commit second
check "skip" "$(next skip)" ""
check "patch" "$(next patch)" v0.1.1
check "minor" "$(next minor)" v0.2.0
check "major" "$(next major)" v1.0.0

git tag v0.1.9
commit third
git tag v0.1.10
git tag not-a-version
git tag v0.2.0-rc.1
commit fourth
check "数値で比べ、semver 以外のタグは無視する" "$(next patch)" v0.1.11

if next huge 2>/dev/null; then
  echo "FAIL: 知らない桁を受け付けた" >&2
  failed=1
fi

commit fifth
git tag v0.1.11
git checkout -q HEAD~1
if next patch 2>/dev/null; then
  echo "FAIL: 最新のタグより古いコミットで番号を出した" >&2
  failed=1
fi

exit "$failed"
