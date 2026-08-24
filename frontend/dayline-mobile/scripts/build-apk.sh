#!/usr/bin/env bash
# Rebundle and build a testing APK of dayline-mobile on demand.
#
# The JS bundle is baked with the values in .env.production (deployed API
# gateway), so the resulting APK talks to the prod instances, not local ones.
# Release builds are signed with the debug keystore (Expo template default),
# which is exactly what sideload testing needs.
#
# Usage:
#   scripts/build-apk.sh            # incremental: reuse android/ if present
#   scripts/build-apk.sh --clean    # regenerate the native project first
#   scripts/build-apk.sh --publish  # build, then upload to the droplet so the
#                                   # APK is served at
#                                   # https://admin.nouveauricheglobalgroup.com/downloads/dayline.apk
#
# Overridable env: JAVA_HOME, ANDROID_HOME, DAYLINE_DEPLOY_HOST (ssh alias,
# default do-sa-user), DAYLINE_DEPLOY_DIR (default
# ~/services/nouveauricheglobalgroup/downloads).
set -euo pipefail

APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$APP_DIR"

CLEAN=0
PUBLISH=0
for arg in "$@"; do
  case "$arg" in
    --clean) CLEAN=1 ;;
    --publish) PUBLISH=1 ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done

# Toolchain. AGP for this RN version wants JDK 17-21; the system default JDK
# may be newer, so prefer 21 when the caller has not pinned one.
if [ -z "${JAVA_HOME:-}" ] && [ -d /usr/lib/jvm/java-21-openjdk ]; then
  export JAVA_HOME=/usr/lib/jvm/java-21-openjdk
fi
export ANDROID_HOME="${ANDROID_HOME:-$HOME/Android/Sdk}"
if [ ! -d "$ANDROID_HOME" ]; then
  echo "ANDROID_HOME=$ANDROID_HOME does not exist; install the Android SDK or point ANDROID_HOME at it" >&2
  exit 1
fi
echo "JAVA_HOME=${JAVA_HOME:-system default}"
echo "ANDROID_HOME=$ANDROID_HOME"

# Release bundles must pick up .env.production (Expo CLI dotenv convention).
export NODE_ENV=production
grep -E '^EXPO_PUBLIC_API_BASE_URL=' .env.production

if [ ! -d node_modules ]; then
  echo "node_modules missing; running npm ci"
  npm ci
fi

if [ "$CLEAN" -eq 1 ] && [ -d android ]; then
  rm -rf android
fi
if [ ! -d android ]; then
  echo "generating native android project (expo prebuild)"
  npx expo prebuild --platform android --no-install
fi

echo "building release APK"
(cd android && ./gradlew --no-daemon assembleRelease)

APK_SRC=android/app/build/outputs/apk/release/app-release.apk
[ -f "$APK_SRC" ] || { echo "expected APK not found: $APK_SRC" >&2; exit 1; }

VERSION="$(node -p "require('./package.json').version")"
GITSHA="$(git rev-parse --short HEAD 2>/dev/null || echo nogit)"
mkdir -p dist
VERSIONED="dist/dayline-${VERSION}-${GITSHA}.apk"
cp "$APK_SRC" "$VERSIONED"
cp "$APK_SRC" dist/dayline.apk
echo "built: $VERSIONED ($(du -h "$VERSIONED" | cut -f1))"

if [ "$PUBLISH" -eq 1 ]; then
  HOST="${DAYLINE_DEPLOY_HOST:-do-sa-user}"
  DIR="${DAYLINE_DEPLOY_DIR:-services/nouveauricheglobalgroup/downloads}"
  echo "publishing to $HOST:$DIR"
  ssh "$HOST" "mkdir -p $DIR"
  rsync --partial -z "$VERSIONED" "$HOST:$DIR/$(basename "$VERSIONED")"
  # Refresh the stable download name atomically on the remote side.
  ssh "$HOST" "cp $DIR/$(basename "$VERSIONED") $DIR/dayline.apk.new && mv $DIR/dayline.apk.new $DIR/dayline.apk"
  echo "published: https://admin.nouveauricheglobalgroup.com/downloads/dayline.apk"
  echo "versioned: https://admin.nouveauricheglobalgroup.com/downloads/$(basename "$VERSIONED")"
fi
