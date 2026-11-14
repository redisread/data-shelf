#!/usr/bin/env bash
set -euo pipefail

go tool mygo build -platform darwin/arm64
app=build/darwin-arm64/data-shelf.app
codesign --verify --deep --strict --verbose=2 "$app"
test "$(lipo -archs "$app/Contents/MacOS/data-shelf")" = arm64
mkdir -p build/ci
ditto -c -k --sequesterRsrc --keepParent "$app" build/ci/data-shelf-arm64.app.zip
for dmg in build/darwin-arm64/*.dmg; do
  hdiutil verify "$dmg"
  cp "$dmg" build/ci/
done
{
  printf 'commit=%s\n' "$(git rev-parse HEAD)"
  printf 'go=%s\n' "$(go version)"
  printf 'target=darwin/arm64\nsigning=ad-hoc (not notarized)\n'
} > build/ci/build-info.txt
(cd build/ci && shasum -a 256 ./*.zip ./*.dmg build-info.txt > SHA256SUMS)
