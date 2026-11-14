# Third-party components

Lookout's original code is Elastic-2.0. Third-party components retain their licenses.

- `@tendstead/ui`: MIT; [notice](tendstead-ui-MIT.txt). Its shell labels are modified
  locally by `web/scripts/patch-shared-ui.mjs`; that modification is not an upstream release.
- `pgx/v5` and its pgxpool support packages: MIT/BSD notices included in the image's
  generated Go dependency notice bundle.
- Svelte, Melt UI and other bundled JavaScript dependencies: their installed
  license texts are collected during the frontend build.

The production image carries these notices in `/app/third-party/`. Build/test-only
packages are not shipped as runtime services. Metadata/data/artwork providers will
have separate attribution/access requirements when those adapters are implemented.
