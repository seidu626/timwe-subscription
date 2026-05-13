# Session Capsule: codex-tmp060-sentry-error-handler

Task: `TMP-060`
Status: `done`

## Summary

Fixed the webspa-admin startup crash by registering Sentry's Angular error handler through createErrorHandler instead of using SentryErrorHandler as an Angular DI class provider.

## Completed Work

- Created and classified TMP-060.
- Replaced the Sentry ErrorHandler class provider with Sentry.createErrorHandler().
- Disabled external font CSS inlining for production builds so Angular does not need network access to fetch Google Fonts.
- Verified the dev runtime path that produced the reported injector error.

## Unfinished Work


## Next Tasks

