<script lang="ts">
  import { onMount } from 'svelte';
  import { AppShell, type Destination } from '@tendstead/ui';
  import { translate, storedLocale, type Locale, type Key } from './lib/i18n';
  import Icon from './lib/Icon.svelte';
  import LibraryIcon from './lib/LibraryIcon.svelte';
  import SystemIcon from './lib/SystemIcon.svelte';
  let locale = $state<Locale>('en');
  let dark = $state(false);
  let pathname = $state(window.location.pathname);
  let checking = $state(true);
  let service = $state<boolean | null>(null);
  let database = $state<boolean | null>(null);
  let checked = $state<Date | null>(null);
  const t = (key: Key) => translate(locale, key);
  const destinations = $derived<Destination[]>([
    { id: 'overview', label: t('overview'), href: '/', icon: LibraryIcon },
    { id: 'system', label: t('system'), href: '/system', icon: SystemIcon },
  ]);
  const isSystem = $derived(pathname === '/system');
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
    document.title = `${isSystem ? t('system') : t('overview')} · Lookout`;
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
  pageTitle={isSystem ? t('system') : t('overview')}
  {pathname}
  {destinations}
  mobilePrimaryIds={['overview', 'system']}
  onNavigate={navigate}
  {locale}
>
  {#snippet pageActions()}
    <div class="preferences">
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
  <div class="page">
    {#if !isSystem}
      <section class="intro">
        <p class="eyebrow">{t('local')}</p>
        <h2>{t('headline')}</h2>
        <p class="lead">{t('intro')}</p>
      </section>
      <section class="empty-card" aria-labelledby="library-title">
        <span class="empty-icon"><Icon kind="library" size={32} /></span><span
          class="tag">{t('foundation')}</span
        >
        <h3 id="library-title">{t('empty')}</h3>
        <p>{t('emptyText')}</p>
        <div class="empty-note"><Icon kind="eye" size={17} />{t('footer')}</div>
      </section>
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
    {#if !isSystem}
      <section class="roadmap">
        <div class="section-heading">
          <h3>{t('next')}</h3>
          <span class="muted">{t('planned')}</span>
        </div>
        <ol>
          {#each [{ title: 'scan', text: 'scanText' }, { title: 'catalogue', text: 'catalogueText' }, { title: 'follow', text: 'followText' }] as item, i}<li
            >
              <span class="step">0{i + 1}</span>
              <div>
                <h4>{t(item.title as Key)}</h4>
                <p>{t(item.text as Key)}</p>
              </div>
            </li>{/each}
        </ol>
      </section>
    {:else}
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
