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
  await expect(
    view.getByText('Latest aired season: 4', { exact: true }),
  ).toBeVisible();
  await expect(
    view.getByText('Local seasons: 1, 2, 3', { exact: true }),
  ).toBeVisible();
  await expect(
    view.getByText('0 / 10 aired episodes present', { exact: true }),
  ).toBeVisible();
  await view.getByRole('button', { name: /Partial seasons/ }).click();
  await expect(
    view.getByText('3 / 10 aired episodes present', { exact: true }),
  ).toBeVisible();
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
