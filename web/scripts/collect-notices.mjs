import { readdirSync, readFileSync, mkdirSync, writeFileSync } from 'node:fs';
import { resolve, relative, join } from 'node:path';
const modules = resolve('node_modules');
const out = resolve('../.implementation/notices');
mkdirSync(out, { recursive: true });
const chunks = [
  'Third-party frontend dependency license texts. Generated from locked npm installation.\n',
];
for (const entry of readdirSync(modules, {
  recursive: true,
  withFileTypes: true,
})) {
  if (
    entry.isFile() &&
    /^(licen[sc]e|copying|notice)(\.|$)/i.test(entry.name)
  ) {
    const path = join(entry.parentPath, entry.name);
    chunks.push(
      `\n--- ${relative(modules, path)} ---\n${readFileSync(path, 'utf8')}\n`,
    );
  }
}
writeFileSync(join(out, 'frontend.txt'), chunks.join(''));
