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
