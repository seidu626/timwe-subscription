# TMP-047 Notes

- Reviewed `services/landing-web/package.json`, `next.config.js`, and `middleware.ts` before package edits.
- `npm audit --audit-level=moderate` reported Next and PostCSS advisories; npm's automatic fix proposed a breaking Next 16 upgrade.
- Context7 Next.js v16 docs identify Node 20.9+ and React 19 as upgrade requirements. The current shell runtime is Node 24, which satisfies the Node floor.
- The first Next 16 build compiled but failed type-checking because App Router dynamic `params` are Promise-backed in route/page contexts. The implementation scope was narrowed to the affected dynamic landing-web files and `tsconfig.json`.
