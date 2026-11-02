#!/usr/bin/env bash
set -euo pipefail

tag="${1:?usage: changelog.sh <tag>}"
prev="$(git describe --tags --abbrev=0 "${tag}^" 2>/dev/null || true)"
range="${prev:+$prev..}$tag"

declare -A titles=(
  [feat]="Features" [fix]="Fixes" [perf]="Performance" [refactor]="Refactors"
  [test]="Tests" [docs]="Docs" [chore]="Chores" [ci]="CI" [build]="Build"
)
order=(feat fix perf refactor test docs ci build chore other)

log="$(git log --no-merges --pretty='%h%x09%s' "$range")"

echo "## ${tag}"
echo
echo "> **Go rewrite.** This build is the \`mood\` CLI only (\`mood analyze --json\`). The Python REST API, GUI and library are not included; they live on \`main\`."
echo
[ -n "$prev" ] && { echo "Changes since ${prev}."; echo; }

for type in "${order[@]}"; do
  lines=""
  while IFS=$'\t' read -r hash subject; do
    [ -z "$hash" ] && continue
    if [[ "$subject" =~ ^([a-z]+)(\([^\)]*\))?!?:[[:space:]]*(.*)$ ]] && [[ -n "${titles[${BASH_REMATCH[1]}]:-}" ]]; then
      t="${BASH_REMATCH[1]}"; msg="${BASH_REMATCH[3]}"
    else
      t="other"; msg="$subject"
    fi
    [ "$t" = "$type" ] && lines+="- ${msg} (${hash})"$'\n'
  done <<<"$log"
  [ -z "$lines" ] && continue
  echo "### ${titles[$type]:-Other}"
  printf '%s\n' "$lines"
done
