import { describe, it, expect } from 'vitest';
import { translations, translate, storedLocale } from './i18n';
describe('translations', () => {
  it('has matching keys', () =>
    expect(Object.keys(translations.de).sort()).toEqual(
      Object.keys(translations.en).sort(),
    ));
  it('translates reactively selected language', () => {
    expect(translate('en', 'overview')).toBe('Overview');
    expect(translate('de', 'overview')).toBe('Übersicht');
  });
  it('validates stored values', () => {
    expect(storedLocale('xx')).toBe('en');
    expect(storedLocale('de')).toBe('de');
  });
});
