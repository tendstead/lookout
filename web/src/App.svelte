<script lang="ts">
  import { onMount } from 'svelte';
  import { AppShell, type Destination } from '@tendstead/ui';
  import { translate, storedLocale, type Locale, type Key } from './lib/i18n';
  import Icon from './lib/Icon.svelte';
  import LibraryIcon from './lib/LibraryIcon.svelte';
  import ServerIcon from './lib/SystemIcon.svelte';
  import WatchlistIcon from './lib/WatchlistIcon.svelte';
  import CalendarIcon from './lib/CalendarIcon.svelte';
  import SourcesIcon from './lib/SourcesIcon.svelte';
  import SettingsIcon from './lib/SettingsIcon.svelte';
  import Library from './lib/Library.svelte';
  import Connections from './lib/Connections.svelte';
  import Releases from './lib/Releases.svelte';
  import Servers from './lib/Servers.svelte';
  import Wanted from './lib/Wanted.svelte';
  let locale = $state<Locale>('en');
  let dark = $state(false);
  let limitWidth = $state(false);
  let pathname = $state(window.location.pathname);
  // Match the AppShell breakpoint so desktop and mobile expose the same routes.
  const mobileLayout = window.matchMedia('(max-width: 767px)');
  let compactNavigation = $state(mobileLayout.matches);
  let checking = $state(true);
  let readOnly = $state(true);
  let trackingReadOnly = $state(true);
  let service = $state<boolean | null>(null);
  let database = $state<boolean | null>(null);
  let checked = $state<Date | null>(null);
  const t = (key: Key) => translate(locale, key);
  const destinations = $derived<Destination[]>([
    { id: 'overview', label: t('overview'), href: '/', icon: LibraryIcon },
    ...(compactNavigation
      ? [
          {
            id: 'tracking',
            label: t('tracking'),
            href: '/wanted',
            icon: WatchlistIcon,
          },
        ]
      : [
          {
            id: 'watchlist',
            label: t('watchlist'),
            href: '/wanted',
            icon: WatchlistIcon,
          },
          {
            id: 'releases',
            label: t('releases'),
            href: '/releases',
            icon: CalendarIcon,
          },
        ]),
    { id: 'servers', label: t('servers'), href: '/servers', icon: ServerIcon },
    { id: 'sources', label: t('sources'), href: '/sources', icon: SourcesIcon },
    {
      id: 'settings',
      label: t('settings'),
      href: '/settings',
      icon: SettingsIcon,
    },
  ]);
  const isWanted = $derived(pathname === '/wanted');
  const isServers = $derived(pathname === '/servers');
  const isSources = $derived(pathname === '/sources');
  const isSettings = $derived(
    ['/settings', '/system', '/settings/connections', '/connections'].includes(
      pathname,
    ),
  );
  const isReleases = $derived(pathname === '/releases');
  const isTracking = $derived(isWanted || isReleases);
  const isDetail = $derived(pathname.startsWith('/titles/'));
  const isGallery = $derived(isReleases || pathname === '/');
  const pageTitle = $derived(
    t(
      isTracking
        ? compactNavigation
          ? 'tracking'
          : isWanted
            ? 'watchlist'
            : 'releases'
        : isSettings
          ? 'settings'
          : isServers
            ? 'servers'
            : isSources
              ? 'sources'
              : isDetail
                ? 'library'
                : 'overview',
    ),
  );
  function toggleWidth() {
    limitWidth = !limitWidth;
    save('lookout.limitWidth', String(limitWidth));
  }
  function save(key: string, value: string) {
    try {
      localStorage.setItem(key, value);
    } catch {
      /* preferences remain usable when storage is unavailable */
    }
  }
  function changeLocale(value: Locale) {
    locale = value;
    save('lookout.locale', value);
  }
  function toggleTheme() {
    dark = !dark;
    save('lookout.theme', dark ? 'dark' : 'light');
  }
  $effect(() => {
    document.documentElement.lang = locale;
    document.documentElement.dataset.theme = dark ? 'dark' : 'light';
    document.title = pageTitle + ' · Lookout';
  });
  $effect(() => {
    if (isSettings) void check();
  });
  async function check() {
    checking = true;
    const probe = async (path: string) => {
      try {
        const response = await fetch(path, {
          cache: 'no-store',
          signal: AbortSignal.timeout(4000),
        });
        return response.ok;
      } catch {
        return false;
      }
    };
    [service, database] = await Promise.all([
      probe('/healthz'),
      probe('/readyz'),
    ]);
    checked = new Date();
    checking = false;
  }
  async function loadMode() {
    try {
      const response = await fetch('/api/v1/status', {
        cache: 'no-store',
        signal: AbortSignal.timeout(4000),
      });
      if (response.ok) {
        const mode = await response.json();
        readOnly = mode.read_only !== false;
        trackingReadOnly = mode.tracking_read_only ?? readOnly;
      }
    } catch {
      // Keep the UI read-only if the deployment mode is unavailable.
    }
  }
  onMount(() => {
    try {
      locale = storedLocale(localStorage.getItem('lookout.locale'));
      dark = localStorage.getItem('lookout.theme') === 'dark';
      limitWidth = localStorage.getItem('lookout.limitWidth') === 'true';
    } catch {
      /* use defaults */
    }
    void loadMode();
    const pop = () => {
      pathname = window.location.pathname;
    };
    const updateNavigation = () => {
      compactNavigation = mobileLayout.matches;
    };
    window.addEventListener('popstate', pop);
    mobileLayout.addEventListener('change', updateNavigation);
    return () => {
      window.removeEventListener('popstate', pop);
      mobileLayout.removeEventListener('change', updateNavigation);
    };
  });
  function navigate(event: MouseEvent, href: string) {
    if (
      event.defaultPrevented ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey
    )
      return;
    event.preventDefault();
    if (pathname === href) return;
    history.pushState({}, '', href);
    pathname = href;
    window.scrollTo(0, 0);
  }
</script>

<AppShell
  brand="Tendstead"
  appLabel="Lookout"
  {pageTitle}
  pathname={isTracking && compactNavigation
    ? '/wanted'
    : isSettings
      ? '/settings'
      : pathname}
  {destinations}
  mobilePrimaryIds={['overview', 'tracking', 'sources']}
  onNavigate={(event, destination) => navigate(event, destination.href)}
  {locale}
>
  {#snippet pageActions()}
    <div class="preferences">
      {#if isGallery}<button
          class="width-button"
          aria-label={t('limitWidth')}
          aria-pressed={limitWidth}
          title={t('limitWidthHint')}
          onclick={toggleWidth}
          ><Icon kind="library" size={18} /><span>{t('limitWidth')}</span
          ></button
        >{/if}
      <div class="language" role="group" aria-label={t('language')}>
        <button
          class:active={locale === 'en'}
          aria-pressed={locale === 'en'}
          onclick={() => changeLocale('en')}>EN</button
        ><button
          class:active={locale === 'de'}
          aria-pressed={locale === 'de'}
          onclick={() => changeLocale('de')}>DE</button
        >
      </div>
      <button
        class="icon-button"
        aria-label={dark ? t('themeLight') : t('themeDark')}
        onclick={toggleTheme}
        ><Icon kind={dark ? 'sun' : 'moon'} size={19} /></button
      >
    </div>
  {/snippet}
  <div
    class="page"
    class:gallery-page={isGallery}
    class:overview-page={pathname === '/'}
    class:width-limited={isGallery && limitWidth}
  >
    {#if isTracking}
      {#if compactNavigation}<nav
          class="view-nav"
          aria-label={t('trackingViews')}
        >
          <a href="/wanted" aria-current={isWanted ? 'page' : undefined}
            >{t('watchlist')}</a
          >
          <a href="/releases" aria-current={isReleases ? 'page' : undefined}
            >{t('releases')}</a
          >
        </nav>{/if}
      {#if isWanted}<Wanted
          {locale}
          readOnly={trackingReadOnly}
          showHeading={compactNavigation}
        />
      {:else}<Releases
          {locale}
          readOnly={trackingReadOnly}
          showHeading={compactNavigation}
        />{/if}
    {:else if isSettings}
      <Connections {locale} {readOnly} />
      <section class="settings-system" aria-labelledby="system-title">
        <section class="catalogue-intro">
          <h2 id="system-title">{t('system')}</h2>
          <p>{t('systemIntro')}</p>
        </section>
        <section class="status-card" aria-labelledby="status-title">
          <div class="section-heading">
            <h3 id="status-title">{t('status')}</h3>
            <button
              class="text-button"
              disabled={checking}
              onclick={() => void check()}
              ><Icon kind="refresh" size={16} />{checking
                ? t('checkingButton')
                : t('refresh')}</button
            >
          </div>
          <div class="status-grid" aria-live="polite">
            <div class="status-item">
              <span>{t('service')}</span><strong
                class:unavailable={service === false}
                ><i class:off={service === false}></i>{service === null
                  ? t('checking')
                  : service
                    ? t('available')
                    : t('unavailable')}</strong
              >
            </div>
            <div class="status-item">
              <span>{t('database')}</span><strong
                data-testid="database-status"
                class:unavailable={database === false}
                ><i class:off={database === false}></i>{database === null
                  ? t('checking')
                  : database
                    ? t('connected')
                    : t('unavailable')}</strong
              >
            </div>
          </div>
          <p class="checked">
            {t('lastChecked')}: {checked
              ? new Intl.DateTimeFormat(locale, {
                  hour: '2-digit',
                  minute: '2-digit',
                  second: '2-digit',
                }).format(checked)
              : t('notChecked')}
          </p>
        </section>
        <section class="details-card">
          <h3>{t('architecture')}</h3>
          <p>{t('architectureText')}</p>
          <dl>
            <div>
              <dt>{t('stage')}</dt>
              <dd>{t('stageValue')}</dd>
            </div>
            <div>
              <dt>{t('api')}</dt>
              <dd>
                <a href="/api/v1/status"
                  >/api/v1/status <span aria-hidden="true">↗</span></a
                >
              </dd>
            </div>
          </dl>
        </section>
      </section>
    {:else if isServers}
      <Servers {locale} {readOnly} />
    {:else}
      {#if !isSources && !isDetail}
        <section class="intro">
          <p class="eyebrow">{t('local')}</p>
          <h2>{t('headline')}</h2>
          <p class="lead">{t('intro')}</p>
        </section>
      {/if}
      {#key pathname}
        <Library
          {locale}
          {readOnly}
          {trackingReadOnly}
          mode={isSources ? 'sources' : isDetail ? 'detail' : 'overview'}
          titleID={isDetail ? pathname.slice('/titles/'.length) : ''}
        />
      {/key}
    {/if}
  </div>
</AppShell>

<style>
  .settings-system {
    margin-block-start: 32px;
  }
  .view-nav {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    margin-bottom: 16px;
  }
  .view-nav a {
    color: var(--ts-text);
    border: 1px solid var(--ts-border);
    border-radius: 8px;
    padding: 8px 14px;
    text-decoration: none;
  }
  .view-nav a[aria-current='page'] {
    background: var(--ts-selection);
    border-color: var(--ts-primary);
    color: var(--ts-on-selection);
  }
</style>
