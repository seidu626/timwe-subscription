const fs = require('fs');
const path = require('path');

// Dynamic Expo config layered on top of app.json (Expo passes the static
// config in as `config`). Push notification setup lives here because the
// Firebase config files are operator-supplied artifacts that are NOT checked
// in: referencing a missing googleServicesFile breaks `expo prebuild`, so each
// platform's entry is wired only when the file actually exists in the project
// root. Drop-in paths (see docs/dayline-push-setup.md):
//   ./GoogleService-Info.plist  (iOS)
//   ./google-services.json      (Android)
const IOS_GOOGLE_SERVICES = './GoogleService-Info.plist';
const ANDROID_GOOGLE_SERVICES = './google-services.json';

function fileExists(relative) {
  return fs.existsSync(path.resolve(__dirname, relative));
}

module.exports = ({ config }) => {
  const ios = { ...config.ios };
  const android = { ...config.android };

  if (fileExists(IOS_GOOGLE_SERVICES)) {
    ios.googleServicesFile = IOS_GOOGLE_SERVICES;
  }
  if (fileExists(ANDROID_GOOGLE_SERVICES)) {
    android.googleServicesFile = ANDROID_GOOGLE_SERVICES;
  }

  return {
    ...config,
    ios,
    android,
    // expo-notifications needs its config plugin for correct native push
    // entitlements/channels in prebuilds; the JS API alone is not enough.
    plugins: [...(config.plugins ?? []), 'expo-notifications'],
  };
};
