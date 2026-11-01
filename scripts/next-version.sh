#!/usr/bin/env bash
# 次にリリースするバージョンのタグを標準出力に書く。
# 引数には上げる桁（patch、minor、major）か、リリースしないことを表す skip を渡す。
# タグがまだなければ v0.1.0 を返し、skip なら何も書かない。
# HEAD にもうタグがあれば、skip でもそのタグを返す。失敗したリリースをやり直すときに番号を飛ばさないため。
set -euo pipefail

bump="${1:-patch}"
case "$bump" in
  patch | minor | major | skip) ;;
  *)
    echo "上げる桁には patch、minor、major、skip のどれかを渡してください: $bump" >&2
    exit 1
    ;;
esac

semver='^v[0-9]+\.[0-9]+\.[0-9]+$'

current="$(git tag --points-at HEAD --list 'v*' --sort=-v:refname | grep -E "$semver" | head -n 1 || true)"
if [ -n "$current" ]; then
  echo "$current"
  exit 0
fi
if [ "$bump" = skip ]; then
  exit 0
fi

latest="$(git tag --list 'v*' --sort=-v:refname | grep -E "$semver" | head -n 1 || true)"
if [ -z "$latest" ]; then
  echo v0.1.0
  exit 0
fi

# 古いコミットの実行をやり直したときに、新しいコードを含まない版を大きな番号で出さないため
if ! git merge-base --is-ancestor "$latest" HEAD; then
  echo "HEAD は最新のタグ $latest より古いので、リリースしません" >&2
  exit 1
fi

IFS=. read -r major minor patch <<<"${latest#v}"
case "$bump" in
  patch) patch=$((patch + 1)) ;;
  minor) minor=$((minor + 1)) patch=0 ;;
  major) major=$((major + 1)) minor=0 patch=0 ;;
esac
echo "v$major.$minor.$patch"
