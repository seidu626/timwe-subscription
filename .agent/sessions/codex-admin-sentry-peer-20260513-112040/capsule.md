# Session Capsule: codex-admin-sentry-peer-20260513-112040

Task: `TMP-064`
Status: `done`

## Summary

Resolved the admin UI npm ERESOLVE failure by replacing deprecated @sentry/angular-ivy with @sentry/angular, whose peer dependency range supports the Angular 18 admin app.

## Completed Work

- Created TMP-064 classified defect issue and work order.
- Identified root cause: @sentry/angular-ivy@7.120.4 peers @angular/common/core/router only through <=17.x while webspa-admin uses Angular 18.2.x.
- Replaced the admin dependency with @sentry/angular@10.52.0, whose peer range is >=14.x <=21.x.
- Updated Sentry imports in the standalone bootstrap and legacy NgModule file.
- Regenerated package-lock.json without legacy peer dependency flags.

## Unfinished Work


## Next Tasks
