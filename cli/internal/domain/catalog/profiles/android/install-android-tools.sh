#!/usr/bin/env bash
# Android profile — installs Android SDK cmdline-tools, platform-tools, and Gradle.
#
# This runs as devuser in a build layer after every module, so java-temurin is
# already installed: the block is emitted after renderEnvironmentBlocks and
# executed through a login shell, which sources ~/.dc-env.sh.
set -euo pipefail

ANDROID_ROOT=/opt/android-sdk
GRADLE_ROOT=/opt/gradle
GRADLE_VERSION="8.12"
CMDLINE_TOOLS_URL="https://dl.google.com/android/repository/commandlinetools-linux-11076708_latest.zip"
GRADLE_URL="https://services.gradle.org/distributions/gradle-${GRADLE_VERSION}-bin.zip"

log() { echo "==> $*"; }

require() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "ERROR: $1 is required by the android profile but is not installed" >&2
        exit 1
    fi
}

require java

log "Setting up Android SDK directories at ${ANDROID_ROOT}"
sudo mkdir -p "${ANDROID_ROOT}/cmdline-tools" "${GRADLE_ROOT}"

log "Downloading Android Command-line Tools"
curl -fsSL "${CMDLINE_TOOLS_URL}" -o /tmp/cmdline-tools.zip
sudo unzip -q /tmp/cmdline-tools.zip -d "${ANDROID_ROOT}/cmdline-tools"
sudo mv "${ANDROID_ROOT}/cmdline-tools/cmdline-tools" "${ANDROID_ROOT}/cmdline-tools/latest"
rm -f /tmp/cmdline-tools.zip

log "Downloading Gradle ${GRADLE_VERSION}"
curl -fsSL "${GRADLE_URL}" -o /tmp/gradle.zip
sudo unzip -q /tmp/gradle.zip -d "${GRADLE_ROOT}"
sudo ln -sf "${GRADLE_ROOT}/gradle-${GRADLE_VERSION}/bin/gradle" /usr/local/bin/gradle
rm -f /tmp/gradle.zip

log "Setting devuser permissions on Android SDK and Gradle directories"
sudo chown -R devuser:devuser "${ANDROID_ROOT}" "${GRADLE_ROOT}"

SDKMANAGER="${ANDROID_ROOT}/cmdline-tools/latest/bin/sdkmanager"

log "Accepting Android SDK licenses"
yes | "${SDKMANAGER}" --licenses >/dev/null 2>&1 || true

log "Installing platform-tools (adb), build-tools, and Android 34 platforms"
"${SDKMANAGER}" "platform-tools" "build-tools;34.0.0" "platforms;android-34"

log "Configuring Android and Gradle environment variables"
sudo tee /etc/profile.d/android.sh >/dev/null <<'EOF'
export ANDROID_HOME=/opt/android-sdk
export ANDROID_SDK_ROOT=/opt/android-sdk
export GRADLE_HOME=/opt/gradle
export PATH="$PATH:/opt/android-sdk/cmdline-tools/latest/bin:/opt/android-sdk/platform-tools:/opt/android-sdk/build-tools/34.0.0"
EOF

if [ -f "$HOME/.dc-env.sh" ]; then
    cat <<'EOF' >> "$HOME/.dc-env.sh"
export ANDROID_HOME=/opt/android-sdk
export ANDROID_SDK_ROOT=/opt/android-sdk
export PATH="$PATH:/opt/android-sdk/cmdline-tools/latest/bin:/opt/android-sdk/platform-tools:/opt/android-sdk/build-tools/34.0.0"
EOF
fi

log "Android SDK & Gradle toolchain installed:"
echo "    sdkmanager -> $(command -v "${SDKMANAGER}")"
echo "    gradle     -> $(command -v gradle)"
if [ -x "${ANDROID_ROOT}/platform-tools/adb" ]; then
    echo "    adb        -> ${ANDROID_ROOT}/platform-tools/adb"
fi
gradle --version
