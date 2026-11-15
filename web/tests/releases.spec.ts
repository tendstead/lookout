import { test, expect } from '@playwright/test';
test('release feed distinguishes upcoming schedules from reported premieres', async ({
  page,
}) => {
  await page.route('**/api/v1/releases?*', (route) =>
    route.fulfill({
      json: {
        data: {
          following: 2,
          ready: 2,
          unmatched: 0,
          upcoming_total: 1,
          recent_total: 1,
          limit: 48,
          offset: 0,
          upcoming: [
            {
              title_id: 'anime',
              title: 'Example Anime',
              name: 'Next episode',
              category: 'anime_series',
              season: 2,
              number: 2,
              scheduled_at: '2026-12-01T20:00:00Z',
              precision: 'time',
              url: 'https://www.tvmaze.com/episodes/2',
            },
          ],
          recent: [
            {
              id: 1,
              title_id: 'anime',
              title: 'Example Anime',
              name: 'Anime premiere',
              kind: 'season',
              season: 2,
              number: 1,
              reported_at: '2026-11-14T20:00:00Z',
              precision: 'time',
              delivery: 'held',
              url: 'https://www.tvmaze.com/episodes/1',
            },
          ],
        },
        progress: { running: false, allowed: true },
      },
    }),
  );
  await page.goto('/releases');
  await expect(
    page.getByRole('heading', { name: 'Upcoming episodes', exact: true }),
  ).toBeVisible();
  await expect(page.getByText('Anime premiere', { exact: true })).toBeVisible();
  await expect(page.getByText('New season', { exact: true })).toBeVisible();
  await expect(
    page.getByText(/not confirmation of a local copy/),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});
test('Telegram connection verification has no send-test action', async ({
  page,
}) => {
  let verified = false;
  let checks = 0;
  await page.route('**/api/v1/connections', (route) =>
    route.fulfill({
      json: {
        items: [],
        capabilities: {
          connections_ready: true,
          public_metadata_enabled: false,
        },
        progress: { running: false },
      },
    }),
  );
  await page.route('**/api/v1/sources', (route) =>
    route.fulfill({ json: { items: [] } }),
  );
  await page.route('**/api/v1/telegram/status', (route) =>
    route.fulfill({
      json: { configured: true, verified, delivery_enabled: false },
    }),
  );
  await page.route('**/api/v1/telegram/verify', (route) => {
    checks++;
    verified = true;
    return route.fulfill({
      json: { configured: true, verified: true, delivery_enabled: false },
    });
  });
  await page.goto('/connections');
  await page
    .getByRole('button', { name: 'Verify Telegram connection', exact: true })
    .click();
  await expect(page.getByTestId('telegram-verified')).toHaveText('Verified');
  expect(checks).toBe(1);
  await expect(page.getByRole('button', { name: /send.*test/i })).toHaveCount(
    0,
  );
});

test('missing seasons compare local inventory and keep partial and uncertain titles separate', async ({
  page,
}) => {
  await page.route('**/api/v1/titles/example/artwork', (route) =>
    route.fulfill({
      contentType: 'image/svg+xml',
      body: '<svg xmlns="http://www.w3.org/2000/svg" width="200" height="300"><rect width="200" height="300" fill="#6750a4"/></svg>',
    }),
  );
  const requested: string[] = [];
  await page.route('**/api/v1/releases/gaps?*', (route) => {
    const url = new URL(route.request().url());
    const kind = url.searchParams.get('kind') || 'missing';
    requested.push(kind);
    const q = url.searchParams.get('q') || '';
    return route.fulfill({
      json: {
        items:
          q === 'nothing'
            ? []
            : [
                {
                  title_id: 'example',
                  title:
                    kind === 'review' ? 'Uncertain Anime' : 'Example Series',
                  category: kind === 'review' ? 'anime_series' : 'series',
                  kind,
                  season: kind === 'review' ? 0 : 4,
                  latest_aired_season: 4,
                  local_seasons: [1, 2, 3],
                  aired: 10,
                  present: kind === 'partial' ? 3 : 0,
                  missing_episodes:
                    kind === 'partial'
                      ? [4, 5, 6, 7, 8, 9, 10]
                      : [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
                  reasons:
                    kind === 'review'
                      ? ['anime_ordering', 'unknown_numbering']
                      : [],
                  checked_at: '2026-11-15T00:00:00Z',
                  inventory_at: '2026-11-15T00:00:00Z',
                },
              ],
        total: q === 'nothing' ? 0 : 1,
        missing: 1,
        partial: 1,
        review: 1,
        limit: 48,
        offset: 0,
      },
    });
  });
  await page.goto('/releases');
  const view = page.getByRole('region', {
    name: 'Seasons missing from your library',
    exact: true,
  });
  await expect(view.getByText('Example Series', { exact: true })).toBeVisible();
  const card = view.locator('.gap-list > li').first();
  const art = card.getByRole('img', {
    name: 'Series artwork for Example Series',
    exact: true,
  });
  await expect(art).toBeVisible();
  await expect(art).toHaveAttribute('src', '/api/v1/titles/example/artwork');
  const coverage = card.getByRole('progressbar', {
    name: 'Aired episode coverage',
    exact: true,
  });
  await expect(coverage).toHaveAttribute('aria-valuenow', '0');
  await expect(coverage).toHaveAttribute('aria-valuemax', '10');
  const titleBox = await view
    .getByText('Example Series', { exact: true })
    .boundingBox();
  const artBox = await art.boundingBox();
  expect(titleBox!.width).toBeGreaterThan(100);
  expect(artBox!.height).toBeGreaterThan(100);
  await expect(
    view.getByText('Latest aired season: 4', { exact: true }),
  ).toBeVisible();
  await expect(
    view.getByRole('group', { name: 'Local seasons: 1, 2, 3', exact: true }),
  ).toBeVisible();
  await expect(
    view.getByText('0 / 10 aired episodes present', { exact: true }),
  ).toBeVisible();
  await view.getByRole('button', { name: /Partial seasons/ }).click();
  await expect(
    view.getByText('3 / 10 aired episodes present', { exact: true }),
  ).toBeVisible();
  await expect(coverage).toHaveAttribute('aria-valuenow', '3');
  await card
    .getByText('Missing aired episode numbers (7)', { exact: true })
    .click();
  await expect(
    card.getByRole('group', {
      name: 'Missing aired episode numbers',
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    card
      .getByRole('group', {
        name: 'Missing aired episode numbers',
        exact: true,
      })
      .getByText('4', { exact: true }),
  ).toBeVisible();
  const episodeTiles = card.getByRole('group', {
    name: 'Missing aired episode numbers',
    exact: true,
  });
  await episodeTiles.focus();
  await expect(episodeTiles).toBeFocused();
  const localChips = card.getByRole('group', {
    name: 'Local seasons: 1, 2, 3',
    exact: true,
  });
  await localChips.focus();
  await expect(localChips).toBeFocused();
  await view.getByRole('button', { name: /Needs review/ }).click();
  await expect(
    view.getByText('Uncertain Anime', { exact: true }),
  ).toBeVisible();
  await expect(
    view.getByText('Anime season ordering needs verification.', {
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    view.getByText('Some local files have unknown episode numbering.', {
      exact: true,
    }),
  ).toBeVisible();
  await expect(view.getByText(/aired episodes present/)).toHaveCount(0);
  await expect(view.getByRole('progressbar')).toHaveCount(0);
  expect(requested).toEqual(
    expect.arrayContaining(['missing', 'partial', 'review']),
  );
  await view.getByRole('searchbox').fill('nothing');
  await view.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(
    view.getByText('No matching series need review.', { exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});

test('season gap polling returns to a valid page when results shrink', async ({
  page,
}) => {
  await page.clock.install();
  let shrink = false;
  await page.route('**/api/v1/releases/gaps?*', (route) => {
    const offset = Number(
      new URL(route.request().url()).searchParams.get('offset'),
    );
    const row = {
      title_id: 'one',
      title: offset ? 'Last gap' : 'First gap',
      category: 'series',
      kind: 'missing',
      season: 4,
      latest_aired_season: 4,
      local_seasons: [1, 2, 3],
      aired: 1,
      present: 0,
      missing_episodes: [1],
      reasons: [],
      checked_at: null,
      inventory_at: null,
    };
    return route.fulfill({
      json: {
        items: shrink && offset ? [] : [row],
        total: shrink ? 48 : 49,
        missing: shrink ? 48 : 49,
        partial: 0,
        review: 0,
        limit: 48,
        offset,
      },
    });
  });
  await page.goto('/releases');
  const view = page.getByRole('region', {
    name: 'Seasons missing from your library',
    exact: true,
  });
  await expect(view.getByText('First gap', { exact: true })).toBeVisible();
  await view.getByRole('button', { name: 'Next', exact: true }).click();
  await expect(view.getByText('Last gap', { exact: true })).toBeVisible();
  shrink = true;
  await page.clock.fastForward(30100);
  await expect(view.getByText('First gap', { exact: true })).toBeVisible();
  await expect(
    view.getByText('No missing seasons found in the comparable series.', {
      exact: true,
    }),
  ).toHaveCount(0);
});

test('season artwork failure keeps visual coverage and title usable', async ({
  page,
}) => {
  await page.route('**/api/v1/releases/gaps?*', (route) =>
    route.fulfill({
      json: {
        items: [
          {
            title_id: 'no-art',
            title: 'A long series title with no artwork available',
            category: 'series',
            kind: 'missing',
            season: 12,
            latest_aired_season: 12,
            local_seasons: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11],
            aired: 3,
            present: 0,
            missing_episodes: [1, 2, 3],
            reasons: [],
            checked_at: null,
            inventory_at: null,
          },
        ],
        total: 1,
        missing: 1,
        partial: 0,
        review: 0,
        limit: 48,
        offset: 0,
      },
    }),
  );
  await page.route('**/api/v1/titles/no-art/artwork', (route) =>
    route.fulfill({ status: 404, body: 'Not available' }),
  );
  await page.goto('/releases');
  const view = page.getByRole('region', {
    name: 'Seasons missing from your library',
    exact: true,
  });
  await expect(view.getByText('No artwork', { exact: true })).toBeVisible();
  await expect(
    view.getByRole('link', {
      name: 'A long series title with no artwork available',
      exact: true,
    }),
  ).toHaveAttribute('href', '/titles/no-art');
  await expect(view.getByRole('progressbar')).toHaveAttribute(
    'aria-valuemax',
    '3',
  );
  await page.getByRole('button', { name: 'Use dark theme' }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.getByRole('button', { name: 'DE', exact: true }).click();
  await expect(
    page.getByRole('heading', {
      name: 'Fehlende Staffeln in deiner Mediathek',
      exact: true,
    }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});

test('series cards merge seasons and reflow as the same window is resized', async ({
  page,
}) => {
  let latestOnly = false;
  await page.route('**/api/v1/releases/gaps?*', (route) => {
    const grouped =
      new URL(route.request().url()).searchParams.get('group_by') === 'title';
    const seasons = (latestOnly ? [9] : [2, 3, 4, 5, 6, 7, 8, 9]).map(
      (season) => ({
        season,
        kind: 'missing',
        aired: season === 9 ? 7 : 10,
        present: 0,
        missing_episodes:
          season === 9
            ? [1, 2, 3, 4, 5, 6, 7]
            : [1, 2, 3, 4, 5, 6, 7, 8, 9, 10],
      }),
    );
    const common = {
      title: 'SpongeBob SquarePants',
      title_id: 'sponge',
      category: 'series',
      kind: 'missing',
      latest_aired_season: 9,
      local_seasons: [1],
      reasons: [],
      checked_at: null,
      inventory_at: null,
    };
    const items = grouped
      ? [
          {
            ...common,
            season: 0,
            aired: 77,
            present: 0,
            missing_episodes: [],
            seasons,
          },
        ]
      : seasons.map((season) => ({ ...common, ...season }));
    const more = Array.from({ length: 5 }, (_, i) => ({
      ...common,
      ...seasons[0],
      title_id: 'other-' + i,
      title: 'Other series ' + i,
      ...(grouped ? { season: 0, seasons: [seasons[0]] } : {}),
    }));
    return route.fulfill({
      json: {
        items: [...items, ...more],
        total: grouped ? 6 : 13,
        missing: 13,
        partial: 0,
        review: 0,
        limit: 48,
        offset: 0,
      },
    });
  });
  await page.route('**/api/v1/titles/*/artwork', (route) =>
    route.fulfill({ status: 404, body: 'No artwork' }),
  );
  await page.clock.install();
  await page.goto('/releases');
  const view = page.getByRole('region', {
    name: 'Seasons missing from your library',
    exact: true,
  });
  await expect(
    view.getByRole('link', { name: 'SpongeBob SquarePants', exact: true }),
  ).toHaveCount(1);
  const card = view.locator('.gap-list > li').first();
  await expect(
    card.getByRole('button', { name: 'Season 9', exact: true }),
  ).toBeVisible();
  await card.getByRole('button', { name: 'Season 9', exact: true }).click();
  await expect(card.getByRole('progressbar')).toHaveAttribute(
    'aria-valuemax',
    '7',
  );
  await card.getByRole('button', { name: 'Season 2', exact: true }).click();
  await expect(card.getByRole('progressbar')).toHaveAttribute(
    'aria-valuemax',
    '10',
  );
  latestOnly = true;
  await page.clock.fastForward(30100);
  await expect(card.getByRole('progressbar')).toHaveAttribute(
    'aria-valuemax',
    '7',
  );
  const columns: number[] = [];
  for (const width of [1800, 1280, 1000, 390]) {
    await page.setViewportSize({ width, height: 900 });
    const first = view.locator('.gap-list > li').first();
    await first.scrollIntoViewIfNeeded();
    const count = await view.locator('.gap-list > li').evaluateAll((nodes) => {
      const first = nodes[0].getBoundingClientRect();
      return nodes.filter(
        (n) => Math.abs(n.getBoundingClientRect().top - first.top) < 2,
      ).length;
    });
    columns.push(count);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
  }
  expect(columns[0]).toBeGreaterThanOrEqual(3);
  expect(columns[1]).toBe(2);
  expect(columns[2]).toBe(1);
  expect(columns[3]).toBe(1);
});
