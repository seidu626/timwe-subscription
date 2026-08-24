# TMP-082: Dayline OTP auto-fill

## Problem

The OTP screen requires manually typing each digit. On-device testing shows
the SMS arriving (Truecaller popup with a Copy OTP button) while the app
offers no assistance: no keyboard autofill suggestion, no paste handling
(each box discards all but the last typed character), and no auto-submit.

## Approach

Zero-native-module autofill. The SMS Retriever API needs the 11-char app
hash appended to the SMS body, which is not controllable for delegated
Arkesel OTP sends, so instead:

1. `OtpInput` boxes get `autoComplete="sms-otp"` (Android autofill hint,
   surfaces the one-tap code chip from Google keyboard/Messages) and
   `textContentType="oneTimeCode"` (iOS keyboard suggestion). The first box
   additionally handles multi-character insertions: when autofill or paste
   delivers the whole code into one box, digits are distributed across all
   boxes instead of being truncated to one.
2. Clipboard assist: on mount and whenever the app returns to the
   foreground on this screen, read the clipboard (expo-clipboard); when it
   holds a six-digit code, show a "Use code NNN NNN" chip. Tapping fills
   and submits. Covers the Truecaller "Copy OTP" flow exactly.
3. Auto-submit once six digits are present (from any entry path), with the
   error path clearing the code and refocusing the first box.

## Non-goals

- SMS Retriever/User Consent native modules (needs SMS template control or
  an extra permission dialog per read).
- Backend/OTP template changes.

## Verification

- `cd frontend/dayline-mobile && npx tsc --noEmit` clean.
- Manual device smoke: keyboard chip appears for the incoming SMS; Copy OTP
  then reopening the app surfaces the clipboard chip; six digits
  auto-verify.
