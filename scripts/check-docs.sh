#!/bin/sh
# Verify documentation health: every relative Markdown link resolves, and
# every page under docs/ is reachable from the docs README index.
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root"

errors=$(mktemp)
links=$(mktemp)
pages=$(mktemp)
trap 'rm -f "$errors" "$links" "$pages"' EXIT INT TERM

add_error() {
	printf '%s\n' "$*" >>"$errors"
}

# Collect every Markdown file we care about (docs plus the repository README)
# exactly once.
{
	find docs -type f -name '*.md' | sort
	[ -f README.md ] && printf '%s\n' README.md
} | sort -u >"$pages"

while IFS= read -r file; do
	dir=$(dirname "$file")
	# Inline link targets: ](path) and ](path "title").
	grep -oE '\]\([^)]*\)' "$file" 2>/dev/null | sed -e 's/^](//' -e 's/)$//' >"$links" || true
	while IFS= read -r target; do
		[ -n "$target" ] || continue
		# Drop an optional Markdown title and any fragment.
		target=${target%% *}
		target=${target%%#*}
		[ -n "$target" ] || continue
		case "$target" in
		http://* | https://* | mailto:* | '#'*) continue ;;
		esac
		case "$target" in
		/*) path=".$target" ;;
		*) path="$dir/$target" ;;
		esac
		[ -e "$path" ] || add_error "$file: broken link: $target"
	done <"$links"
done <"$pages"

# Every docs page except the index itself must be linked from docs/README.md.
if [ -f docs/README.md ]; then
	while IFS= read -r page; do
		case "$page" in
		docs/README.md | README.md) continue ;;
		esac
		rel=${page#docs/}
		if ! grep -Fq "($rel)" docs/README.md && ! grep -Fq "(./$rel)" docs/README.md; then
			add_error "docs/README.md: missing index link to $rel"
		fi
	done <"$pages"
else
	add_error "docs/README.md: file is missing"
fi

if [ -s "$errors" ]; then
	echo "documentation check failed:"
	cat "$errors"
	exit 1
fi
echo "documentation check passed"
