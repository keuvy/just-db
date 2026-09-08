#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != Darwin ]]; then
  echo "DMG packaging requires macOS." >&2
  exit 1
fi

create_dmg=${CREATE_DMG:-create-dmg}
if ! command -v "$create_dmg" >/dev/null 2>&1; then
  echo "Missing create-dmg. Install it with: brew install create-dmg" >&2
  exit 1
fi

if [[ "${1:-}" == --check ]]; then
  exit 0
fi
if [[ $# != 1 || ! "$1" =~ ^[a-zA-Z0-9][a-zA-Z0-9._+-]*$ ]]; then
  echo "Usage: $0 <version>" >&2
  exit 1
fi

packaging_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_dir=$(cd "$packaging_dir/../../.." && pwd)
app="$repo_dir/desktop/build/bin/just-db.app"
output_dir="$repo_dir/dist"
output="$output_dir/just-db-$1.dmg"

if [[ ! -d "$app" ]]; then
  echo "Missing $app. Build it with make desktop first." >&2
  exit 1
fi

mkdir -p "$output_dir"
work_dir=$(mktemp -d "$output_dir/.dmg.XXXXXX")
trap 'rm -rf "$work_dir"' EXIT
mkdir "$work_dir/source"
# Stage just the app, excluding stale binaries or other build outputs.
ditto "$app" "$work_dir/source/just-db.app"

# Coordinates match the 660 x 440 background. Window height includes the title bar.
"$create_dmg" \
  --volname "just-db Installer" \
  --volicon "$app/Contents/Resources/iconfile.icns" \
  --background "$packaging_dir/background.png" \
  --window-pos 200 120 \
  --window-size 660 468 \
  --icon-size 84 \
  --text-size 13 \
  --icon "just-db.app" 180 250 \
  --hide-extension "just-db.app" \
  --app-drop-link 480 250 \
  --format UDZO \
  "$work_dir/installer.dmg" \
  "$work_dir/source"

# Keep the previous release intact if creating the replacement fails.
mv -f "$work_dir/installer.dmg" "$output"
echo "Created $output"
