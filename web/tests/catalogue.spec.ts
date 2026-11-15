import { test, expect } from '@playwright/test';
test('catalogue browsing searches, filters and opens observed files', async ({
  page,
}) => {
  const titles = [
    {
      id: 'anime-id',
      name: 'Space Anime',
      category: 'anime_series',
      provider_ids: {},
      provenance: 'filename',
      match_status: 'unmatched',
      file_count: 2,
      source_label: 'Anime',
    },
    {
      id: 'film-id',
      name: 'Example Film',
      category: 'movie',
      year: 2020,
      provider_ids: {},
      provenance: 'filename',
      match_status: 'unmatched',
      file_count: 1,
      source_label: 'Films',
    },
  ];
  await page.route('**/api/v1/titles?*', async (route) => {
    const u = new URL(route.request().url());
    const q = u.searchParams.get('q')?.toLowerCase() || '';
    const category = u.searchParams.get('category');
    const items = titles.filter(
      (t) =>
        t.name.toLowerCase().includes(q) &&
        (!category || t.category === category),
    );
    await route.fulfill({
      json: { items, total: items.length, limit: 48, offset: 0 },
    });
  });
  await page.route('**/api/v1/sources', (route) =>
    route.fulfill({
      json: {
        items: [
          {
            id: 'series',
            label: 'Anime',
            category: 'anime_series',
            path: '/media/anime',
            file_count: 3,
            status: 'ready',
            stats: {},
          },
        ],
      },
    }),
  );
  await page.route('**/api/v1/titles/anime-id', (route) =>
    route.fulfill({
      json: {
        ...titles[0],
        files: [
          {
            id: 1,
            relative_path: 'Space Anime/Season 1/Space.S01E02.mkv',
            size: 1000,
            season: 1,
            episodes: [2],
            numbering: 'season_episode',
            observation: 'stable',
          },
          {
            id: 2,
            relative_path: 'Space Anime/absolute-128.mkv',
            size: 2000,
            season: null,
            episodes: [],
            numbering: 'unknown',
            observation: 'observed',
          },
        ],
      },
    }),
  );
  await page.route('**/api/v1/titles/anime-id/follow', (route) =>
    route.fulfill({
      json: { enabled: false, initialized: false, provider_id: '9' },
    }),
  );
  await page.goto('/');
  await expect(page.getByRole('link', { name: /Space Anime/ })).toBeVisible();
  await page.getByRole('searchbox', { name: 'Search library' }).fill('Space');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByRole('link', { name: /Example Film/ })).toHaveCount(0);
  await page.getByRole('button', { name: 'Anime series', exact: true }).click();
  await page.getByRole('link', { name: /Space Anime/ }).click();
  await expect(
    page.getByRole('heading', { name: 'Space Anime', exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText('Space Anime/absolute-128.mkv', { exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});
test('metadata connections support both server choices without exposing saved tokens', async ({
  page,
}) => {
  let saved = false;
  await page.route('**/api/v1/connections', async (route) => {
    if (route.request().method() === 'POST') {
      const input = route.request().postDataJSON();
      expect(input.token).toBe('private-fixture-token');
      saved = true;
      await route.fulfill({
        status: 201,
        json: {
          id: 'new',
          kind: input.kind,
          url: input.url,
          token_configured: true,
        },
      });
    } else {
      await route.fulfill({
        json: {
          items: saved
            ? [
                {
                  id: 'new',
                  kind: 'jellyfin',
                  url: 'http://media.test:8096',
                  path_mappings: [],
                  verified: true,
                  token_configured: true,
                },
              ]
            : [],
          capabilities: {
            connections_ready: true,
            public_metadata_enabled: true,
          },
          progress: { running: false },
        },
      });
    }
  });
  await page.route('**/api/v1/sources', (route) =>
    route.fulfill({ json: { items: [] } }),
  );
  await page.goto('/connections');
  await expect(
    page.getByRole('heading', { name: 'Metadata connections', exact: true }),
  ).toBeVisible();
  await page.getByLabel('Server type').selectOption('plex');
  await page.getByLabel('Server type').selectOption('jellyfin');
  await page.getByLabel('Media server URL').fill('http://media.test:8096');
  await page.getByLabel('Access token').fill('private-fixture-token');
  await page
    .getByRole('button', { name: 'Save connection', exact: true })
    .click();
  await expect(page.getByLabel('Access token')).toHaveValue('');
  await expect(
    page.getByText('http://media.test:8096', { exact: true }),
  ).toBeVisible();
  expect(await page.locator('body').innerText()).not.toContain(
    'private-fixture-token',
  );
});
