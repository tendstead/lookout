// Keep fresh Go checkouts buildable before frontend assets are generated.
import { writeFileSync } from 'node:fs';
writeFileSync(new URL('../../cmd/server/static/.gitkeep', import.meta.url), '');
