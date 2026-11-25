import { test, expect } from '@playwright/test';
test('foundation is usable, bilingual and persists preferences', async ({
  page,
}, testInfo) => {
  await page.goto('/');
  await expect(
    page.getByRole('heading', { name: 'Your media, in view.' }),
  ).toBeVisible();
  await expect(page.getByTestId('database-status')).toHaveCount(0);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.getByRole('button', { name: 'DE', exact: true }).click();
  await expect(
    page.getByRole('heading', { name: 'Deine Medien im Blick.' }),
  ).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole('heading', { name: 'Deine Medien im Blick.' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'EN', exact: true }).click();
  await page.getByRole('button', { name: 'Use dark theme' }).click();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.reload();
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await page.getByRole('button', { name: 'Use light theme' }).click();
  await expect(page.getByTestId('database-status')).toHaveCount(0);
  if (process.env.UPDATE_SCREENSHOTS === '1') {
    if (testInfo.project.name === 'mobile') {
      await page.goto('/releases');
      const upcoming = page.getByRole('region', {
        name: 'Upcoming episodes',
        exact: true,
      });
      await expect(
        upcoming.getByRole('heading', {
          name: 'Upcoming episodes',
          exact: true,
        }),
      ).toBeVisible();
      const firstCard = upcoming.locator('.upcoming-card').first();
      if (await firstCard.count()) {
        await firstCard.evaluate((el) => el.scrollIntoView({ block: 'start' }));
        await firstCard.evaluate(async (el) => {
          const img = el.querySelector('img');
          if (img && !img.complete)
            await new Promise<void>((resolve) => {
              img.addEventListener('load', () => resolve(), { once: true });
              img.addEventListener('error', () => resolve(), { once: true });
            });
        });
      } else
        await upcoming.evaluate((el) => el.scrollIntoView({ block: 'start' }));
    } else {
      await page.setViewportSize({ width: 1920, height: 1000 });
      const grid = page.locator('.title-grid');
      if (await grid.count()) {
        await page.evaluate(() => window.scrollTo(0, 0));
        await page.waitForFunction(() =>
          [...document.querySelectorAll<HTMLImageElement>('.title-art img')]
            .filter((img) => {
              const rect = img.getBoundingClientRect();
              return rect.top < innerHeight && rect.bottom > 0;
            })
            .every((img) => img.complete),
        );
      }
    }
    await page.screenshot({
      path: `../docs/screenshots/${testInfo.project.name}.png`,
      fullPage: false,
      scale: 'css',
    });
  }
});
test('Settings shows Connections before System status on one page', async ({
  page,
}) => {
  let checks = 0;
  await page.route('**/readyz', (route) => {
    checks++;
    return route.fulfill({ json: { status: 'ready' } });
  });
  await page.goto('/');
  await expect(
    page.getByRole('heading', { name: 'Your media, in view.' }),
  ).toBeVisible();
  await expect(page.getByTestId('database-status')).toHaveCount(0);
  expect(checks).toBe(0);
  for (const path of [
    '/settings',
    '/connections',
    '/system',
    '/settings/connections',
  ]) {
    await page.goto(path);
    await expect(
      page.getByRole('main').getByRole('heading', {
        name: /^(Metadata connections|Service status)$/,
      }),
    ).toHaveText(['Metadata connections', 'Service status']);
    await expect(page.getByTestId('database-status')).toHaveText('Connected');
    await expect(page.getByRole('main').getByRole('navigation')).toHaveCount(0);
  }
});

test('an unavailable database is reported in System', async ({ page }) => {
  await page.route('**/readyz', (route) =>
    route.fulfill({
      status: 503,
      contentType: 'application/json',
      body: '{"status":"unavailable"}',
    }),
  );
  await page.goto('/settings');
  await expect(page.getByTestId('database-status')).toHaveText('Unavailable');
  await expect(
    page.getByRole('button', { name: 'DE', exact: true }),
  ).toBeVisible();
});
test('API never returns the SPA', async ({ request }) => {
  const r = await request.get('/api/v1/does-not-exist');
  expect(r.status()).toBe(404);
  expect(r.headers()['content-type']).toContain('application/json');
  expect((await r.json()).error.code).toBe('not_found');
});

test('shell controls and navigation follow the selected language', async ({
  page,
}, info) => {
  await page.goto('/');
  await page.getByRole('button', { name: 'DE', exact: true }).click();
  if (info.project.name === 'mobile') {
    await page.getByRole('button', { name: 'Menü', exact: true }).click();
    await expect(
      page.getByRole('button', { name: 'Menü schließen' }),
    ).toBeVisible();
  }
  await page
    .getByRole('link', { name: 'Einstellungen', exact: true })
    .filter({ visible: true })
    .click();
  await expect(
    page.getByRole('heading', { name: 'Dienststatus', exact: true }).first(),
  ).toBeVisible();
  await page.reload();
  await expect(page).toHaveURL(/\/settings$/);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});

test('desktop separates Watchlist and Releases and mobile keeps Tracking', async ({
  page,
}, info) => {
  await page.goto('/wanted');
  const subnav = page.getByRole('navigation', { name: 'Tracking views' });
  if (info.project.name === 'mobile') {
    const mobile = page.getByRole('navigation', { name: 'Mobile navigation' });
    await expect(
      mobile.getByRole('link', { name: 'Tracking' }),
    ).toHaveAttribute('aria-current', 'page');
    await subnav.getByRole('link', { name: 'Releases' }).click();
    await expect(
      subnav.getByRole('link', { name: 'Releases' }),
    ).toHaveAttribute('aria-current', 'page');
    return;
  }
  const nav = page.getByRole('navigation', { name: 'Main navigation' });
  const watchlist = nav.getByRole('link', { name: 'Watchlist', exact: true });
  const releases = nav.getByRole('link', { name: 'Releases', exact: true });
  await expect(watchlist).toHaveAttribute('aria-current', 'page');
  await expect(subnav).toHaveCount(0);
  await expect(nav.getByRole('link', { name: 'Tracking' })).toHaveCount(0);
  const icons = await nav
    .locator('a svg')
    .evaluateAll((items) => items.map((icon) => icon.innerHTML));
  expect(icons).toHaveLength(6);
  expect(new Set(icons).size).toBe(6);
  await releases.click();
  await expect(page).toHaveURL(/\/releases$/);
  await expect(releases).toHaveAttribute('aria-current', 'page');
  await expect(
    page.getByRole('heading', { level: 1, name: 'Releases' }),
  ).toBeVisible();
  await page.goBack();
  await expect(watchlist).toHaveAttribute('aria-current', 'page');
});
