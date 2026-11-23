// Local compatibility patch for @tendstead/ui (MIT).
// Remove when the published package exposes translated shell labels.
import { readFileSync, writeFileSync } from 'node:fs';
const root = new URL('../node_modules/@tendstead/ui/', import.meta.url);
const pkg = JSON.parse(readFileSync(new URL('package.json', root), 'utf8'));
const consumer = JSON.parse(
  readFileSync(new URL('../package.json', import.meta.url), 'utf8'),
);
if (pkg.version !== consumer.dependencies['@tendstead/ui'])
  throw new Error(
    'The installed @tendstead/ui must match the version pinned in package.json',
  );
const app = new URL('dist/AppShell.svelte', root);
let s = readFileSync(app, 'utf8');
if (!s.includes("locale='en'")) {
  if (!s.includes('let { brand,'))
    throw new Error('The shared shell prop declaration changed');
  s = s.replace('let { brand,', "let { locale='en', brand,");
  for (const [en, de] of Object.entries({
    'Connected services': 'Verbundene Dienste',
    'Skip to content': 'Zum Inhalt springen',
    'Application sidebar': 'Seitenleiste',
    'Main navigation': 'Hauptnavigation',
    'A little more in order.': 'Ein wenig mehr Ordnung.',
    'Mobile navigation': 'Mobile Navigation',
    Menu: 'Menü',
    'All destinations': 'Alle Bereiche',
    'Close menu': 'Menü schließen',
    'All navigation': 'Alle Bereiche',
  })) {
    if (!s.includes(en)) throw new Error(`Shell text changed: ${en}`);
    s = s
      .replaceAll(`"${en}"`, `{locale==='de'?'${de}':'${en}'}`)
      .replaceAll(`>${en}<`, `>{locale==='de'?'${de}':'${en}'}<`);
  }
  writeFileSync(app, s);
  const types = new URL('dist/types.d.ts', root);
  const t = readFileSync(types, 'utf8').replace(
    'export interface AppShellProps {',
    "export interface AppShellProps {\n    locale?: 'en' | 'de';",
  );
  writeFileSync(types, t);
}
