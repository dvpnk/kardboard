#!/bin/bash
# Print the version of the current commit, such as 0.2.1, from the
# Conventional Commits since the last version tag.
#
# Same rules as calculate_version.sh in the other projects, but it starts
# at the newest tag vX.Y.Z reachable from HEAD instead of at 0.0.0, so the
# history before that tag no longer counts:
#
#   feat!                                major +1
#   feat, fix!, refactor!, build!        minor +1
#   fix, refactor, build, chore!, docs!  patch +1
#   anything else                        nothing
#
# A scope is allowed: feat(ui): and feat(ui)!: count like feat: and feat!:.
# To give a release its version for good: git tag "v$(scripts/calculate_version.sh)"
set -euo pipefail
cd "$(dirname "$0")/.."

major=0
minor=0
patch=0
range=HEAD

if tag=$(git describe --tags --abbrev=0 --match 'v[0-9]*.[0-9]*.[0-9]*' HEAD 2>/dev/null); then
	IFS=. read -r major minor patch <<<"${tag#v}"
	range="$tag..HEAD"
fi

re='^([a-z]+)(\([^)]*\))?(!?):'
while IFS= read -r subject; do
	[[ $subject =~ $re ]] || continue
	type=${BASH_REMATCH[1]}${BASH_REMATCH[3]}
	case $type in
	'feat!')
		((++major))
		minor=0
		patch=0
		;;
	feat | 'fix!' | 'refactor!' | 'build!')
		((++minor))
		patch=0
		;;
	fix | refactor | build | 'chore!' | 'docs!')
		((++patch))
		;;
	esac
done < <(git log --reverse --format=%s "$range")

echo "$major.$minor.$patch"
