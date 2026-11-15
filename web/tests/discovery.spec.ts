import { test, expect } from '@playwright/test';

test('upcoming cards emphasize scheduled dates and distinguish premieres from missing seasons', async ({
  page,
}) => {
  await page.route('**/api/v1/releases?*', (route) =>
    route.fulfill({
      json: {
        data: {
          following: 2,
          ready: 2,
          unmatched: 0,
          upcoming_total: 2,
          recent_total: 0,
          recent: [],
          upcoming: [
            {
              title_id: 'premiere',
              title: 'Future Series',
              name: 'A new beginning',
              season: 3,
              number: 1,
              scheduled_at: '2026-12-02T00:00:00Z',
              air_date: '2026-12-01',
              precision: 'date',
              url: 'https://www.tvmaze.com/episodes/1',
            },
            {
              title_id: 'next',
              title: 'Another Series',
              name: 'Next chapter',
              season: 1,
              number: 5,
              scheduled_at: '2026-12-03T20:00:00Z',
              precision: 'time',
              url: '',
            },
          ],
        },
        progress: { allowed: true, running: false },
      },
    }),
  );
  await page.route('**/api/v1/titles/*/artwork', (route) =>
    route.fulfill({ status: 404, body: 'No artwork' }),
  );
  await page.goto('/releases');
  const upcoming = page.getByRole('region', {
    name: 'Upcoming episodes',
    exact: true,
  });
  await expect(upcoming.locator('.upcoming-card')).toHaveCount(2);
  const premiere = upcoming.locator('.upcoming-card').first();
  await expect(
    premiere.getByText('Season premiere', { exact: true }),
  ).toBeVisible();
  await expect(
    premiere.getByText('Dec 1, 2026', { exact: true }),
  ).toBeVisible();
  await expect(premiere.getByText('Date only', { exact: true })).toBeVisible();
  await expect(
    upcoming
      .locator('.upcoming-card')
      .nth(1)
      .getByText('Upcoming episode', { exact: true }),
  ).toBeVisible();
  await expect(upcoming.getByRole('progressbar')).toHaveCount(0);
  await expect(
    upcoming.getByRole('group', { name: 'Local seasons', exact: false }),
  ).toHaveCount(0);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});

test('overview and releases share fluid card sizing and a browser-local width limit', async ({
  page,
  browser,
}) => {
  await page.setViewportSize({ width: 2600, height: 1000 });
  await page.route('**/api/v1/titles?*', (route) =>
    route.fulfill({
      json: {
        items: Array.from({ length: 12 }, (_, i) => ({
          id: 'title-' + i,
          name: 'Local title ' + i,
          category: 'movie',
          provider_ids: {},
          provenance: 'filename',
          match_status: 'unmatched',
          file_count: 1,
          source_label: 'Films',
        })),
        total: 12,
        limit: 48,
        offset: 0,
      },
    }),
  );
  await page.goto('/');
  await expect(page.locator('.title-card')).toHaveCount(12);
  const count = () =>
    page.locator('.title-card').evaluateAll((nodes) => {
      const top = nodes[0].getBoundingClientRect().top;
      return nodes.filter(
        (node) => Math.abs(node.getBoundingClientRect().top - top) < 2,
      ).length;
    });
  expect(await count()).toBeGreaterThanOrEqual(6);
  const toggle = page.getByRole('button', { name: 'Limit width', exact: true });
  await toggle.click();
  await expect(toggle).toHaveAttribute('aria-pressed', 'true');
  expect(await count()).toBeGreaterThanOrEqual(4);
  expect(await count()).toBeLessThanOrEqual(6);
  expect(
    await page
      .locator('.page')
      .evaluate((el) => el.getBoundingClientRect().width),
  ).toBeLessThanOrEqual(1600);
  await page.reload();
  await expect(toggle).toHaveAttribute('aria-pressed', 'true');
  await page.goto('/releases');
  await expect(toggle).toHaveAttribute('aria-pressed', 'true');
  expect(
    await page
      .locator('.page')
      .evaluate((el) => el.getBoundingClientRect().width),
  ).toBeLessThanOrEqual(1600);
  await toggle.click();
  await expect(toggle).toHaveAttribute('aria-pressed', 'false');
  expect(
    await page
      .locator('.page')
      .evaluate((el) => el.getBoundingClientRect().width),
  ).toBeGreaterThan(1600);
  const fresh = await browser.newContext({
    viewport: { width: 2600, height: 1000 },
  });
  const other = await fresh.newPage();
  await other.goto(new URL('/', page.url()).href);
  await expect(
    other.getByRole('button', { name: 'Limit width', exact: true }),
  ).toHaveAttribute('aria-pressed', 'false');
  await fresh.close();
  await page.setViewportSize({ width: 390, height: 664 });
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});

test('type-ahead suggests only local titles and supports keyboard selection and escape', async ({
  page,
}) => {
  await page.clock.install();
  const queries: string[] = [];
  await page.route('**/api/v1/titles?*', (route) =>
    route.fulfill({ json: { items: [], total: 0, limit: 48, offset: 0 } }),
  );
  await page.route('**/api/v1/titles/suggestions?*', (route) => {
    queries.push(new URL(route.request().url()).searchParams.get('q') || '');
    return route.fulfill({
      json: {
        items: [
          {
            id: 'space',
            name: 'Space Anime',
            category: 'anime_series',
            year: 2020,
          },
          { id: 'film', name: 'Space Film', category: 'movie' },
        ],
      },
    });
  });
  await page.route('**/api/v1/titles/space', (route) =>
    route.fulfill({
      json: {
        id: 'space',
        name: 'Space Anime',
        category: 'anime_series',
        provenance: 'filename',
        provider_ids: {},
        match_status: 'unmatched',
        file_count: 1,
        source_label: 'Anime',
        files: [],
      },
    }),
  );
  await page.goto('/');
  const input = page.getByRole('combobox', {
    name: 'Search library',
    exact: true,
  });
  await input.fill('S');
  await page.clock.fastForward(250);
  expect(queries).toHaveLength(0);
  await input.fill('Sp');
  await page.clock.fastForward(250);
  await expect(page.getByRole('option', { name: /Space Anime/ })).toBeVisible();
  await expect(page.getByRole('listbox').getByRole('option')).toHaveCount(2);
  expect(queries).toEqual(['Sp']);
  await input.press('Escape');
  await expect(page.getByRole('listbox')).toHaveCount(0);
  await input.fill('Space');
  await page.clock.fastForward(250);
  await expect(page.getByRole('listbox').getByRole('option')).toHaveCount(2);
  await input.press('ArrowDown');
  await input.press('Enter');
  await expect(page).toHaveURL(/\/titles\/space$/);
  await page.route('**/api/v1/titles/film', (route) =>
    route.fulfill({
      json: {
        id: 'film',
        name: 'Space Film',
        category: 'movie',
        provenance: 'filename',
        provider_ids: {},
        match_status: 'unmatched',
        file_count: 1,
        source_label: 'Films',
        files: [],
      },
    }),
  );
  await page.goto('/');
  await input.fill('Space');
  await page.clock.fastForward(250);
  await page.getByRole('option', { name: /Space Film/ }).click();
  await expect(page).toHaveURL(/\/titles\/film$/);
});

test('suggestions ignore old responses and leave ordinary search usable on failure', async ({
  page,
}) => {
  await page.clock.install();
  await page.route('**/api/v1/sources', (route) =>
    route.fulfill({
      json: {
        items: [
          {
            id: 'local',
            label: 'Local library',
            path: '/fixture',
            category: 'movie',
            status: 'ready',
            file_count: 2,
            last_success: '2026-11-15T12:00:00Z',
            stats: {},
          },
        ],
      },
    }),
  );
  let old: Parameters<Parameters<typeof page.route>[1]>[0] | undefined;
  await page.route('**/api/v1/titles?*', (route) =>
    route.fulfill({ json: { items: [], total: 0, limit: 48, offset: 0 } }),
  );
  await page.route('**/api/v1/titles/suggestions?*', (route) => {
    const q = new URL(route.request().url()).searchParams.get('q');
    if (q === 'Old') {
      old = route;
      return;
    }
    if (q === 'Fail')
      return route.fulfill({
        status: 503,
        json: { error: { code: 'unavailable' } },
      });
    return route.fulfill({
      json: {
        items: [{ id: 'new', name: 'New local title', category: 'movie' }],
      },
    });
  });
  await page.goto('/');
  const input = page.getByRole('combobox', {
    name: 'Search library',
    exact: true,
  });
  await input.fill('Old');
  await page.clock.fastForward(250);
  await expect.poll(() => !!old).toBe(true);
  await input.fill('New');
  await page.clock.fastForward(250);
  await expect(
    page.getByRole('option', { name: /New local title/ }),
  ).toBeVisible();
  await old
    ?.fulfill({
      json: { items: [{ id: 'old', name: 'Old result', category: 'movie' }] },
    })
    .catch(() => {});
  await expect(page.getByRole('option', { name: /Old result/ })).toHaveCount(0);
  await input.fill('Fail');
  await page.clock.fastForward(250);
  await expect(
    page.getByText('Suggestions unavailable. You can still search.', {
      exact: true,
    }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'No matching titles', exact: true }),
  ).toBeVisible();
});

test('upcoming artwork is retried when a refreshed card represents another series', async ({
  page,
}) => {
  await page.clock.install();
  let replaced = false;
  await page.route('**/api/v1/releases?*', (route) =>
    route.fulfill({
      json: {
        data: {
          following: 1,
          ready: 1,
          unmatched: 0,
          upcoming_total: 1,
          recent_total: 0,
          recent: [],
          upcoming: [
            {
              title_id: replaced ? 'good' : 'bad',
              title: replaced ? 'Available art' : 'Unavailable art',
              name: 'Next episode',
              season: 1,
              number: 2,
              scheduled_at: '2026-12-01T20:00:00Z',
              precision: 'time',
              url: '',
            },
          ],
        },
        progress: { allowed: true, running: false },
      },
    }),
  );
  await page.route('**/api/v1/titles/bad/artwork', (route) =>
    route.fulfill({ status: 404, body: 'No artwork' }),
  );
  await page.route('**/api/v1/titles/good/artwork', (route) =>
    route.fulfill({
      contentType: 'image/svg+xml',
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="200" height="300"><rect width="200" height="300" fill="#6750a4"/></svg>',
    }),
  );
  await page.goto('/releases');
  const upcoming = page.getByRole('region', {
    name: 'Upcoming episodes',
    exact: true,
  });
  await expect(
    upcoming.getByRole('link', { name: 'Unavailable art', exact: true }),
  ).toBeVisible();
  await expect(upcoming.getByRole('img')).toHaveCount(0);
  replaced = true;
  await page.clock.fastForward(10100);
  await expect(
    upcoming.getByRole('link', { name: 'Available art', exact: true }),
  ).toBeVisible();
  await expect(
    upcoming.getByRole('img', {
      name: 'Series artwork for Available art',
      exact: true,
    }),
  ).toBeVisible();
});

test('overview posters are compact at 1920px while the width limit and phone grid stay useful', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1920, height: 1000 });
  await page.route('**/api/v1/titles?*', (route) =>
    route.fulfill({
      json: {
        items: Array.from({ length: 16 }, (_, i) => ({
          id: 'compact-' + i,
          name: 'Local title ' + i,
          category: 'movie',
          provider_ids: {},
          provenance: 'filename',
          match_status: 'unmatched',
          file_count: 1,
          source_label: 'Films',
        })),
        total: 16,
        limit: 48,
        offset: 0,
      },
    }),
  );
  await page.goto('/');
  await expect(page.locator('.title-card')).toHaveCount(16);
  const count = () =>
    page.locator('.title-card').evaluateAll((nodes) => {
      const top = nodes[0].getBoundingClientRect().top;
      return nodes.filter(
        (node) => Math.abs(node.getBoundingClientRect().top - top) < 2,
      ).length;
    });
  const width = await page
    .locator('.title-card')
    .first()
    .evaluate((el) => el.getBoundingClientRect().width);
  expect(width).toBeGreaterThanOrEqual(170);
  expect(width).toBeLessThanOrEqual(210);
  expect(await count()).toBe(8);
  await page.getByRole('button', { name: 'Limit width', exact: true }).click();
  expect(await count()).toBe(6);
  await page.setViewportSize({ width: 390, height: 664 });
  expect(await count()).toBe(2);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});
