#!/bin/sh
set -eu
mkdir -p /notices
go list -m -f '{{.Path}} {{.Dir}}' all | while read -r module directory; do
  [ "$module" = github.com/tendstead/lookout ] && continue
  [ -n "$directory" ] || continue
  for file in "$directory"/LICENSE* "$directory"/COPYING* "$directory"/NOTICE*; do
    if [ -f "$file" ]; then
      printf '\n--- %s / %s ---\n' "$module" "$(basename "$file")"
      cat "$file"
    fi
  done
done > /notices/go.txt
