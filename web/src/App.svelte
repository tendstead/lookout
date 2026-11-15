<script lang="ts">
  import { onMount } from 'svelte';
  import { AppShell, type Destination } from '@tendstead/ui';
  import { translate, storedLocale, type Locale, type Key } from './lib/i18n';
  import Icon from './lib/Icon.svelte';
  import LibraryIcon from './lib/LibraryIcon.svelte';
  import SystemIcon from './lib/SystemIcon.svelte';
  import Library from './lib/Library.svelte';
  import Connections from './lib/Connections.svelte';
  import Releases from './lib/Releases.svelte';
  let locale = $state<Locale>('en');
  let dark = $state(false);
  let limitWidth = $state(false);
  let pathname = $state(window.location.pathname);
  let checking = $state(true);
  let service = $state<boolean | null>(null);
  let database = $state<boolean | null>(null);
  let checked = $state<Date | null>(null);
  const t = (key: Key) => translate(locale, key);
  const destinations = $derived<Destination[]>([
    { id: 'overview', label: t('overview'), href: '/', icon: LibraryIcon },
    {
      id: 'releases',
      label: t('releases'),
      href: '/releases',
      icon: SystemIcon,
    },
    { id: 'sources', label: t('sources'), href: '/sources', icon: SystemIcon },
    {
      id: 'connections',
      label: t('connections'),
      href: '/connections',
      icon: SystemIcon,
    },
    { id: 'system', label: t('system'), href: '/system', icon: SystemIcon },
  ]);
  const isSystem = $derived(pathname === '/system');
  const isSources = $derived(pathname === '/sources');
  const isConnections = $derived(pathname === '/connections');
  const isReleases = $derived(pathname === '/releases');
  const isDetail = $derived(pathname.startsWith('/titles/'));
  const isGallery = $derived(isReleases || pathname === '/');
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
    document.title = `${isSystem ? t('system') : isConnections ? t('connections') : t('overview')} · Lookout`;
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
  onMount(() => {
    try {
      locale = storedLocale(localStorage.getItem('lookout.locale'));
      dark = localStorage.getItem('lookout.theme') === 'dark';
      limitWidth = localStorage.getItem('lookout.limitWidth') === 'true';
    } catch {
      /* use defaults */
    }
    void check();
    const pop = () => {
      pathname = window.location.pathname;
    };
    window.addEventListener('popstate', pop);
    return () => window.removeEventListener('popstate', pop);
  });
  function navigate(event: MouseEvent, destination: Destination) {
    event.preventDefault();
    history.pushState({}, '', destination.href);
    pathname = destination.href;
    window.scrollTo(0, 0);
  }
</script>

<AppShell
  brand="Tendstead"
  appLabel="Lookout"
  pageTitle={isReleases
    ? t('releases')
    : isConnections
      ? t('connections')
      : isSystem
        ? t('system')
        : isSources
          ? t('sources')
          : isDetail
            ? t('library')
            : t('overview')}
  {pathname}
  {destinations}
  mobilePrimaryIds={['overview', 'releases', 'sources']}
  onNavigate={navigate}
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
    class:width-limited={isGallery && limitWidth}
  >
    {#if !isSystem}
      {#if !isSources && !isDetail && !isConnections && !isReleases}<section
          class="intro"
        >
          <p class="eyebrow">{t('local')}</p>
          <h2>{t('headline')}</h2>
          <p class="lead">{t('intro')}</p>
        </section>{/if}
      {#if isReleases}<Releases {locale} />{:else if isConnections}<Connections
          {locale}
        />{:else}{#key pathname}<Library
            {locale}
            mode={isSources ? 'sources' : isDetail ? 'detail' : 'overview'}
            titleID={isDetail ? pathname.slice('/titles/'.length) : ''}
          />{/key}{/if}
    {:else}
      <section class="intro">
        <p class="eyebrow">Lookout</p>
        <h2>{t('status')}</h2>
        <p class="lead">{t('systemIntro')}</p>
      </section>
    {/if}
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
    {#if isSystem}
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
    {/if}
  </div>
</AppShell>
