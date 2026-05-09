# TMP-026 Notes

- `.gitmodules` is present on `origin/main` and maps `frontend/webspa-admin` to `https://github.com/coreui/coreui-free-angular-admin-template.git`.
- The superproject gitlink pins `frontend/webspa-admin` to `2ad95b18ecff4d8b23e5d1b7152975c477d5137a`.
- `git submodule update --init --recursive frontend/webspa-admin` fails because the configured remote does not contain that pinned commit.
- The failed checkout was deinitialized so this slice does not commit a submodule pointer change.

