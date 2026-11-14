#!/usr/bin/env bash
set -euo pipefail

requested=${1-}
previous=${2-}
semver='^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'

if [[ -n "$previous" && ! "$previous" =~ $semver ]]; then
  echo 'Previous release tag is not a supported version' >&2
  exit 1
fi
if [[ -n "$requested" ]]; then
  version=$requested
elif [[ -z "$previous" ]]; then
  version=v0.1.0
else
  [[ "$previous" =~ $semver ]]
  major=${BASH_REMATCH[1]}
  minor=${BASH_REMATCH[2]}
  if [[ ${#minor} -gt 9 ]]; then
    echo 'Minor version exceeds the supported automatic bump range' >&2
    exit 1
  fi
  version="v${major}.$((10#$minor + 1)).0"
fi
if [[ ! "$version" =~ $semver ]]; then
  echo 'Version must be vMAJOR.MINOR.PATCH with optional prerelease/build suffixes' >&2
  exit 1
fi
printf '%s\n' "$version"
