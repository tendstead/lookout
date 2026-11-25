import { test, expect } from '@playwright/test';
test('server names and freshness settings are editable', async ({ page }) => {
  let name = 'Home NAS';
  let hours = 6;
  await page.route('**/api/v1/servers', (r) =>
    r.fulfill({
      json: {
        items: [
          {
            id: 'home',
            name,
            scan_hours: hours,
            stale_hours: 24,
            status: 'ready',
            last_success: null,
          },
        ],
      },
    }),
  );
  await page.route('**/api/v1/servers/home', async (r) => {
    const body = r.request().postDataJSON();
    name = body.name;
    hours = body.scan_hours;
    await r.fulfill({ json: { saved: true } });
  });
  await page.goto('/servers');
  await page.getByLabel('Server name', { exact: true }).fill('Family NAS');
  await page.getByLabel('Scan interval (hours)', { exact: true }).fill('12');
  await page
    .getByRole('button', { name: 'Save settings', exact: true })
    .click();
  await expect(
    page.getByRole('heading', { name: 'Family NAS', exact: true }),
  ).toBeVisible();
  expect(hours).toBe(12);
});
test('tracking previews an external season and shows local copies on both tabs', async ({
  page,
}, info) => {
  const source = 'https://thetvdb.com/index.php/series/severance';
  const episodeSource =
    'https://www.tvmaze.com/episodes/2302265/severance-1x09-the-we-we-are';
  const season = Array.from({ length: 9 }, (_, i) => ({
    id: String(i + 1),
    name: i === 8 ? 'The We We Are' : 'Episode ' + (i + 1),
    season: 1,
    number: i + 1,
    air_date: '2022-04-08',
    servers: i === 8 ? [] : [i < 4 ? 'Home NAS' : 'Family NAS'],
  }));
  const draft = {
    name: 'Severance',
    category: 'series',
    season: 1,
    year: 2022,
    source_url: source,
    provider_ids: { tvmaze: '44933', tvdb: '371980' },
    status: 'available',
    servers: [
      {
        server_id: 'family',
        name: 'Family NAS',
        status: 'available',
        content: ['S01E05', 'S01E06', 'S01E07', 'S01E08'],
      },
    ],
    episodes: season,
  };
  let entries: unknown[] = [];
  await page.route('**/api/v1/status', (route) =>
    route.fulfill({ json: { read_only: false } }),
  );
  await page.route('**/api/v1/wanted/preview', (route) => {
    const input = route.request().postDataJSON().input;
    if (input === episodeSource) {
      return route.fulfill({
        json: {
          draft: {
            ...draft,
            episode: 9,
            episodes: [season[8]],
            alert_mode: 'episode',
          },
          candidates: [],
          existing_id: 'wanted1',
        },
      });
    }
    expect(input).toBe(source);
    return route.fulfill({ json: { draft, candidates: [] } });
  });
  await page.route('**/api/v1/wanted', async (route) => {
    if (route.request().method() === 'POST') {
      const submitted = route.request().postDataJSON();
      expect(submitted.alert_mode).toBe('season');
      expect(submitted.provider_ids).toEqual(draft.provider_ids);
      expect(submitted.source_url).toBe(source);
      expect(submitted.season).toBe(1);
      entries = [{ ...draft, ...submitted, id: 'wanted1' }];
      return route.fulfill({ json: entries[0] });
    }
    return route.fulfill({ json: { items: entries } });
  });
  await page.route('**/api/v1/releases?*', (route) =>
    route.fulfill({
      json: {
        data: {
          upcoming: [],
          following: 1,
          ready: 1,
          unmatched: 0,
          recent: [
            {
              id: 1,
              tracked_id: 'wanted1',
              title_id: '',
              title: 'Severance',
              name: 'The We We Are',
              season: 1,
              number: 9,
              kind: 'episode',
              reported_at: '2022-04-08T00:00:00Z',
              precision: 'timestamp',
              availability: 'missing',
            },
          ],
        },
        progress: { allowed: true, running: false },
      },
    }),
  );
  await page.goto('/wanted');
  await page.getByLabel('Title or link', { exact: true }).fill(source);
  await page.getByRole('button', { name: 'Preview' }).click();
  await expect(page.getByText('8 of 9 episodes available')).toBeVisible();
  await expect(page.getByText('The We We Are', { exact: false })).toBeVisible();
  await page.getByLabel('Library alerts').selectOption('season');
  await page.getByRole('button', { name: 'Track title' }).click();
  await expect(
    page.getByRole('heading', { name: 'Severance (2022)', exact: true }),
  ).toBeVisible();
  await expect(page.locator('.title-card .server-badge')).toContainText(
    'Family NAS',
  );
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page
    .getByRole('navigation', {
      name:
        info.project.name === 'mobile' ? 'Tracking views' : 'Main navigation',
    })
    .getByRole('link', { name: 'Releases' })
    .click();
  await expect(page.getByText('Reported aired', { exact: true })).toBeVisible();
  await expect(
    page.getByText('Missing from library', { exact: true }),
  ).toBeVisible();
  await page
    .getByRole('link', { name: 'Severance', exact: true })
    .and(page.locator('a[href="/wanted?tracked=wanted1"]'))
    .click();
  await expect(
    page.getByRole('heading', { name: 'Severance (2022)', exact: true }),
  ).toBeVisible();
  await page.getByLabel('Title or link', { exact: true }).fill(episodeSource);
  await page.getByRole('button', { name: 'Preview' }).click();
  await expect(
    page.getByText('You already track this title.', { exact: false }),
  ).toBeVisible();
  await expect(page.getByLabel('Episode (optional)')).toHaveValue('');
  await expect(page.getByLabel('Library alerts')).toHaveValue('season');
  await page.getByRole('button', { name: 'Use linked episode' }).click();
  await expect(page.getByLabel('Episode (optional)')).toHaveValue('9');
});

test('tracking keeps a public video link when its metadata is unavailable', async ({
  page,
}) => {
  const source = 'https://www.youtube.com/watch?v=YE7VzlLtp-4';
  let items: unknown[] = [];
  await page.route('**/api/v1/status', (route) =>
    route.fulfill({ json: { read_only: false } }),
  );
  await page.route('**/api/v1/wanted/preview', (route) => {
    expect(route.request().postDataJSON().input).toBe(source);
    return route.fulfill({
      json: {
        draft: {
          name: '',
          category: 'movie',
          source_url: source,
          status: 'uncertain',
          servers: [],
        },
        candidates: [],
      },
    });
  });
  await page.route('**/api/v1/wanted', (route) => {
    if (route.request().method() === 'POST') {
      const draft = route.request().postDataJSON();
      expect(draft.name).toBe('Big Buck Bunny');
      expect(draft.source_url).toBe(source);
      const entry = { ...draft, id: 'bunny', status: 'missing', servers: [] };
      items = [entry];
      return route.fulfill({ json: entry });
    }
    return route.fulfill({ json: { items } });
  });
  await page.goto('/wanted');
  await page.getByLabel('Title or link', { exact: true }).fill(source);
  await page.getByRole('button', { name: 'Preview' }).click();
  await page.getByLabel('Title', { exact: true }).fill('Big Buck Bunny');
  await page.getByRole('button', { name: 'Track title' }).click();
  const entry = page.locator('article').filter({ hasText: 'Big Buck Bunny' });
  await expect(
    entry.getByRole('link', { name: 'Open source link' }),
  ).toHaveAttribute('href', source);
});

test('read-only views show inventory type and hide shared writes', async ({
  page,
}) => {
  await page.route('**/api/v1/status', (r) =>
    r.fulfill({ json: { read_only: true } }),
  );
  await page.route('**/api/v1/servers', (r) =>
    r.fulfill({
      json: {
        items: [
          {
            id: 'home',
            name: 'Home NAS',
            type: 'ssh',
            scan_hours: 6,
            stale_hours: 24,
            status: 'ready',
            last_success: null,
          },
        ],
      },
    }),
  );
  await page.route('**/api/v1/wanted', (r) =>
    r.fulfill({
      json: {
        items: [
          {
            id: 'request',
            name: 'Dune',
            status: 'available',
            servers: [],
          },
        ],
      },
    }),
  );
  await page.goto('/servers');
  await expect(page.getByRole('heading', { name: 'Home NAS' })).toBeVisible();
  await expect(page.getByText('SSH', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Save settings' })).toHaveCount(
    0,
  );
  await page.goto('/wanted');
  await expect(page.getByRole('heading', { name: 'Dune' })).toBeVisible();
  await expect(
    page.getByRole('button', { name: /Track title|Stop tracking/ }),
  ).toHaveCount(0);
});

test('tracking suggests named scopes and parses a typed episode', async ({
  page,
}) => {
  await page.route('**/api/v1/status', (route) =>
    route.fulfill({ json: { read_only: false } }),
  );
  await page.route('**/api/v1/wanted', (route) =>
    route.fulfill({ json: { items: [] } }),
  );
  await page.route('**/api/v1/titles/suggestions?*', (route) => {
    const query = new URL(route.request().url()).searchParams.get('q');
    return route.fulfill({
      json: {
        items:
          query === 'Severance'
            ? [{ id: 'show', name: 'Severance', category: 'series' }]
            : [],
      },
    });
  });
  await page.route('**/api/v1/wanted/preview', (route) => {
    const { input, season, episode } = route.request().postDataJSON();
    expect(input).toBe('Severance');
    return route.fulfill({
      json: {
        draft: {
          name: 'Severance',
          category: 'series',
          season,
          episode,
          status: 'available',
          season_names: { 1: 'Lumon' },
          servers: [],
          episodes: [
            {
              id: '8',
              name: "What's for Dinner?",
              season: 1,
              number: 8,
              servers: ['Demo'],
            },
            {
              id: '9',
              name: 'The We We Are',
              season: 1,
              number: 9,
              servers: [],
            },
            {
              id: '10',
              name: 'Hello, Ms. Cobel',
              season: 2,
              number: 1,
              servers: [],
            },
          ],
        },
      },
    });
  });
  await page.goto('/wanted');
  const title = page.getByLabel('Title or link');
  await title.fill('Severance S01E09');
  await page.getByRole('option', { name: /Severance/ }).click();
  const season = page.getByRole('combobox', { name: 'Season (optional)' });
  const episode = page.getByRole('combobox', { name: 'Episode (optional)' });
  await expect(season).toHaveValue('1');
  await expect(episode).toHaveValue('9');
  await season.click();
  await expect(
    page.getByRole('option', { name: /Season 1 · Lumon.*1 of 2/ }),
  ).toBeVisible();
  await season.press('Escape');
  await episode.click();
  await expect(
    page.getByRole('option', { name: /E09 · The We We Are.*Missing/ }),
  ).toBeVisible();
  await episode.press('ArrowDown');
  await episode.press('Enter');
  await expect(episode).toHaveValue('8');
  await season.fill('7');
  await expect(episode).toHaveValue('');
  await expect(season).toHaveValue('7');
  await title.fill('Severance season 1');
  await page.getByRole('button', { name: 'Preview' }).click();
  await expect(season).toHaveValue('1');
});

test('watchlist cards use server tags and confirm stopping or removing tracking', async ({
  page,
}) => {
  let current = {
    id: 'tracked',
    name: 'Example Film',
    category: 'movie',
    status: 'available',
    artwork_url: '/api/v1/titles/film/artwork',
    servers: [
      {
        server_id: 'demo',
        name: 'Demo NAS',
        status: 'available',
        content: ['Movie'],
      },
    ],
    alert_mode: 'episode',
    paused: false,
  };
  await page.route('**/api/v1/status', (route) =>
    route.fulfill({ json: { read_only: false } }),
  );
  let removed = false;
  await page.route('**/api/v1/wanted', (route) =>
    route.fulfill({ json: { items: removed ? [] : [current] } }),
  );
  await page.route('**/api/v1/wanted/tracked/settings', (route) => {
    current = { ...current, ...route.request().postDataJSON() };
    return route.fulfill({ json: current });
  });
  await page.route('**/api/v1/wanted/tracked/remove', (route) => {
    removed = true;
    return route.fulfill({ json: {} });
  });
  await page.goto('/wanted');
  const card = page.locator('.title-card').filter({ hasText: 'Example Film' });
  await expect(card.locator('.title-art img')).toHaveAttribute(
    'src',
    current.artwork_url,
  );
  await card.getByRole('button', { name: 'Demo NAS', exact: true }).click();
  await expect(
    page
      .locator('#tracking-preview')
      .getByRole('button', { name: 'Demo NAS', exact: true }),
  ).toHaveAttribute('aria-pressed', 'true');
  await page.getByRole('button', { name: 'Save settings' }).click();
  await expect(
    card.getByRole('button', { name: 'Demo NAS', exact: true }),
  ).toHaveAttribute('aria-pressed', 'true');
  await card.getByRole('button', { name: 'Stop tracking' }).click();
  const modal = page.getByRole('dialog');
  await expect(modal).toBeVisible();
  await modal.getByRole('button', { name: 'Cancel' }).click();
  await expect(
    card.getByRole('button', { name: 'Stop tracking' }),
  ).toBeFocused();
  await card.getByRole('button', { name: 'Stop tracking' }).click();
  await modal.getByRole('button', { name: 'Stop tracking' }).click();
  await expect(
    card.getByRole('button', { name: 'Resume tracking' }),
  ).toBeVisible();
  await card.getByRole('button', { name: 'Remove from Watchlist' }).click();
  await expect(
    modal.getByRole('button', { name: 'Remove from Watchlist' }),
  ).toBeVisible();
  expect(removed).toBe(false);
  await modal.getByRole('button', { name: 'Remove from Watchlist' }).click();
  await expect(card).toHaveCount(0);
  expect(removed).toBe(true);
});
