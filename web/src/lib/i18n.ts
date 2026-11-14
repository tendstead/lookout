export const translations = {
  en: {
    overview: 'Overview',
    system: 'System',
    headline: 'Your media, in view.',
    intro: 'One place to keep up with your films, series and public videos.',
    foundation: 'Foundation preview',
    empty: 'Your library starts here',
    emptyText:
      'No sources are connected yet. Media browsing, source scans and subscriptions are coming in the next milestones.',
    status: 'Service status',
    service: 'Lookout service',
    database: 'Database',
    connected: 'Connected',
    available: 'Available',
    unavailable: 'Unavailable',
    checking: 'Checking…',
    refresh: 'Check again',
    checkingButton: 'Checking',
    next: 'What comes next',
    scan: 'Connect your sources',
    scanText: 'Read-only local folders and SFTP inventories.',
    catalogue: 'Bring your library together',
    catalogueText:
      'Match titles and episodes, with uncertain matches ready for review.',
    follow: 'Keep up with new releases',
    followText: 'Follows, public-video discovery and notifications.',
    planned: 'Planned',
    footer: 'Your files stay where they are.',
    themeDark: 'Use dark theme',
    themeLight: 'Use light theme',
    language: 'Language',
    lastChecked: 'Last checked',
    notChecked: 'Not checked yet',
    systemIntro: 'Live checks for this Lookout installation.',
    stage: 'Current milestone',
    stageValue: 'Application foundation',
    architecture: 'Built to stay small',
    architectureText:
      'One service with its interface built in. A dedicated PostgreSQL database stores application data.',
    noSources: 'No connected sources',
    local: 'Self-hosted · Local first',
    roadmap: 'Upcoming milestones',
    api: 'Read-only status API',
  },
  de: {
    overview: 'Übersicht',
    system: 'System',
    headline: 'Deine Medien im Blick.',
    intro: 'Ein Ort für deine Filme, Serien und öffentlichen Videos.',
    foundation: 'Vorschau des Grundgerüsts',
    empty: 'Hier beginnt deine Mediathek',
    emptyText:
      'Noch sind keine Quellen verbunden. Medienübersicht, Quellensuche und Abonnements folgen in den nächsten Meilensteinen.',
    status: 'Dienststatus',
    service: 'Lookout-Dienst',
    database: 'Datenbank',
    connected: 'Verbunden',
    available: 'Verfügbar',
    unavailable: 'Nicht verfügbar',
    checking: 'Wird geprüft…',
    refresh: 'Erneut prüfen',
    checkingButton: 'Wird geprüft',
    next: 'Die nächsten Schritte',
    scan: 'Deine Quellen verbinden',
    scanText: 'Lokale Ordner und SFTP-Bestände mit reinem Lesezugriff.',
    catalogue: 'Deine Mediathek zusammenführen',
    catalogueText:
      'Titel und Folgen zuordnen und unsichere Treffer gemeinsam prüfen.',
    follow: 'Neue Veröffentlichungen verfolgen',
    followText:
      'Beobachtete Titel, neue öffentliche Videos und Benachrichtigungen.',
    planned: 'Geplant',
    footer: 'Deine Dateien bleiben an ihrem Platz.',
    themeDark: 'Dunkles Design verwenden',
    themeLight: 'Helles Design verwenden',
    language: 'Sprache',
    lastChecked: 'Zuletzt geprüft',
    notChecked: 'Noch nicht geprüft',
    systemIntro: 'Aktuelle Prüfungen dieser Lookout-Installation.',
    stage: 'Aktueller Meilenstein',
    stageValue: 'Anwendungsgrundlage',
    architecture: 'Bewusst schlank aufgebaut',
    architectureText:
      'Ein Dienst mit integrierter Oberfläche. Eine eigene PostgreSQL-Datenbank speichert die Anwendungsdaten.',
    noSources: 'Keine verbundenen Quellen',
    local: 'Selbst gehostet · Lokal zuerst',
    roadmap: 'Kommende Meilensteine',
    api: 'Status-API mit Lesezugriff',
  },
};
export type Locale = 'en' | 'de';
export type Key = keyof typeof translations.en;
export function storedLocale(value: string | null): Locale {
  return value === 'de' ? 'de' : 'en';
}
export function translate(locale: Locale, key: Key): string {
  return translations[locale][key];
}
