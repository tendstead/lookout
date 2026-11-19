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
test('wanted entries show the server supplying requested content', async ({
  page,
}) => {
  let entries: unknown[] = [];
  await page.route('**/api/v1/wanted', async (r) => {
    if (r.request().method() === 'POST') {
      const v = r.request().postDataJSON();
      entries = [
        {
          ...v,
          id: 'wanted1',
          status: 'available',
          servers: [
            {
              server_id: 'family',
              name: 'Family NAS',
              status: 'available',
              content: ['Movie'],
            },
          ],
        },
      ];
      await r.fulfill({ json: entries[0] });
    } else await r.fulfill({ json: { items: entries } });
  });
  await page.goto('/wanted');
  await page.getByLabel('Title', { exact: true }).fill('Dune');
  await page.getByLabel('Year (optional)', { exact: true }).fill('2021');
  await page
    .getByRole('button', { name: 'Add to Wanted', exact: true })
    .click();
  await expect(
    page.getByRole('heading', { name: 'Dune (2021)', exact: true }),
  ).toBeVisible();
  await expect(page.locator('.wanted-server')).toContainText('Family NAS');
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
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
    page.getByRole('button', { name: /Add to Wanted|Remove from Wanted/ }),
  ).toHaveCount(0);
});
