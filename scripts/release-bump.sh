#!/usr/bin/env bash
# 前のタグ以降にマージされた PR のラベルから、上げる桁を決めて標準出力に書く。
# 標準入力は 1 行に PR 1 件で、ラベルをカンマで区切る（ラベルのない PR は空行）。
# release:skip の付いた PR は数えない。数える PR がなければ skip を返す。
# 数える PR に release:major があれば major、release:minor があれば minor、どちらもなければ patch。
set -euo pipefail

bump=skip
while IFS= read -r line || [ -n "$line" ]; do
  labels=",$line,"
  case "$labels" in
    *,release:skip,*) continue ;;
  esac
  case "$labels" in
    *,release:major,*) bump="major" ;;
    *,release:minor,*)
      if [ "$bump" != "major" ]; then
        bump="minor"
      fi
      ;;
    *)
      if [ "$bump" = "skip" ]; then
        bump="patch"
      fi
      ;;
  esac
done
echo "$bump"
