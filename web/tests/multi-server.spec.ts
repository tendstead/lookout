import { test, expect, type Page } from '@playwright/test';
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

const linkedCopy = {
  id: 1,
  copy_revision: 'first',
  relative_path: 'Synthetic.S01E02-E03.mkv',
  server_id: 'linked',
  server_name: 'Linked storage',
  season: 1,
  episodes: [2, 3],
  observation: 'observed',
  numbering: 'season_episode',
  size: 4096,
  file_size: 4096,
  technical: {
    width: 1920,
    height: 1080,
    video_codec: 'h264',
    video_bitrate_bps: 4000000,
    hdr: 'SDR',
    audio: [{ language: 'pt-BR', codec: 'aac', bitrate_bps: 128000 }],
    subtitles: [{ language: 'pt' }],
  },
};

const apiCopy = {
  id: 2,
  copy_revision: 'api-first',
  relative_path: 'Synthetic.S01E02.mkv',
  server_id: 'api',
  server_name: 'Media API',
  season: 1,
  episodes: [2],
  observation: 'observed',
  numbering: 'season_episode',
  size: 0,
  file_size: 1073741824,
  technical: {
    width: 3840,
    height: 2160,
    video_codec: 'HEVC',
    video_bitrate_bps: 8000000,
    hdr: 'HDR',
    audio: [{ language: 'pt-BR', codec: 'aac' }, { language: 'German' }],
    subtitles: [{ language: 'und' }],
  },
};

async function routeSyntheticTitle(page: Page) {
  await page.route('**/api/v1/titles/synthetic', (r) =>
    r.fulfill({
      json: {
        id: 'synthetic',
        name: 'Synthetic series',
        category: 'series',
        files: [
          linkedCopy,
          apiCopy,
          {
            ...apiCopy,
            id: 3,
            observation: 'missing',
            reported_path: 'Synthetic.missing.mkv',
          },
          {
            ...apiCopy,
            id: 4,
            observation: 'unconfirmed',
            reported_path: 'Synthetic.unconfirmed.mkv',
          },
        ],
        file_count: 4,
      },
    }),
  );
}

test('comparison stays collapsed and preserves copies across episode choices', async ({
  page,
}) => {
  await routeSyntheticTitle(page);
  await page.goto('/titles/synthetic');
  const comparison = page.locator('details').filter({
    has: page.getByText('Compare files', { exact: true }),
  });
  await expect(comparison).toBeVisible();
  await expect(
    comparison.getByRole('combobox', { name: 'Episode', exact: true }),
  ).toBeHidden();
  await comparison.getByText('Compare files', { exact: true }).click();
  for (const path of ['Synthetic.missing.mkv', 'Synthetic.unconfirmed.mkv']) {
    await expect(comparison.getByText(path, { exact: true })).toHaveCount(0);
    await expect(page.locator('.observed-files').getByText(path)).toBeVisible();
  }
  await expect(
    comparison.getByText('3840 × 2160', { exact: true }),
  ).toBeVisible();
  await expect(comparison.getByText('1 GiB', { exact: true })).toBeVisible();
  await expect(
    comparison.getByText('S01E02–E03', { exact: true }),
  ).toBeVisible();
  const audio = comparison.getByRole('row').filter({
    has: page.getByRole('rowheader', { name: 'Audio', exact: true }),
  });
  await audio.getByText('Stream details', { exact: true }).first().click();
  await audio.getByText('Stream details', { exact: true }).last().click();
  await expect(audio.getByText('128 kb/s', { exact: true })).toBeVisible();
  await expect(
    audio
      .locator('dt')
      .filter({ hasText: /^Bitrate$/ })
      .last()
      .locator('xpath=following-sibling::dd[1]'),
  ).toHaveText('Unknown');
  await comparison
    .getByRole('combobox', { name: 'Episode', exact: true })
    .selectOption('3');
  await expect(
    comparison.getByText('Linked storage', { exact: true }),
  ).toBeVisible();
  await expect(comparison.getByText('Media API', { exact: true })).toHaveCount(
    0,
  );
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
});

test('dismissed copies persist and reappear when restored or revised', async ({
  page,
}) => {
  await routeSyntheticTitle(page);
  let revision = linkedCopy.copy_revision;
  await page.route('**/api/v1/file-differences?*', (r) =>
    r.fulfill({
      json: {
        items: [
          {
            title_id: 'synthetic',
            title_name: 'Synthetic series',
            season: 1,
            episode: 2,
            files: [{ ...linkedCopy, copy_revision: revision }, apiCopy],
          },
        ],
        limit: 50,
        offset: 0,
        has_more: false,
      },
    }),
  );
  await page.goto('/');
  const menu = page.getByRole('button', { name: 'Menu', exact: true });
  if (await menu.isVisible()) await menu.click();
  const hrefs = await page
    .getByRole('navigation')
    .getByRole('link')
    .evaluateAll((links) => links.map((link) => link.getAttribute('href')));
  const wanted = hrefs.indexOf('/wanted');
  expect(wanted).toBeGreaterThanOrEqual(0);
  expect(hrefs[wanted + 1]).toBe('/file-differences');
  await page
    .getByRole('link', { name: 'File differences', exact: true })
    .click();
  const title = page.getByRole('button', { name: /Synthetic series/ });
  const dialog = page.getByRole('dialog');
  await expect(title).toBeVisible();
  await expect(page.getByText('3840 × 2160', { exact: true })).toHaveCount(0);
  await page
    .getByRole('button', { name: 'Show hidden (0)', exact: true })
    .click();
  await expect(
    page.getByText('No hidden copies.', { exact: true }),
  ).toBeVisible();
  await page
    .getByRole('button', { name: 'Show differences', exact: true })
    .click();
  await title.click();
  await expect(dialog.getByText('3840 × 2160', { exact: true })).toBeVisible();
  await page
    .getByRole('button', { name: 'Hide copy', exact: true })
    .first()
    .click();
  await page.reload();
  await expect(
    page.getByText('No file differences to show.', { exact: true }),
  ).toBeVisible();
  await page.goto('/titles/synthetic');
  await page.getByText('Compare files', { exact: true }).click();
  const comparison = page.getByRole('region', {
    name: 'Compare files',
    exact: true,
  });
  await expect(
    comparison.getByText('Linked storage', { exact: true }),
  ).toBeVisible();
  await expect(
    comparison.getByText('Media API', { exact: true }),
  ).toBeVisible();
  await expect(
    comparison.getByRole('button', { name: 'Hide copy', exact: true }),
  ).toHaveCount(0);
  await page.goBack();
  await page
    .getByRole('button', { name: 'Show hidden (1)', exact: true })
    .click();
  await title.click();
  await dialog.getByRole('button', { name: 'Restore', exact: true }).click();
  await expect(
    page.getByText('No hidden copies.', { exact: true }),
  ).toBeVisible();
  await page
    .getByRole('button', { name: 'Show differences', exact: true })
    .click();
  await title.click();
  await page
    .getByRole('button', { name: 'Hide copy', exact: true })
    .first()
    .click();
  revision = 'second';
  await page.reload();
  await expect(title).toBeVisible();
});

test('language flags preserve explicit regions and support keyboard and touch', async ({
  page,
}, info) => {
  await page.route('**/api/v1/file-differences?*', (r) =>
    r.fulfill({
      json: {
        items: [
          {
            title_id: 'synthetic',
            title_name: 'Synthetic series',
            files: [linkedCopy, apiCopy],
          },
        ],
        limit: 50,
        offset: 0,
        has_more: false,
      },
    }),
  );
  await page.goto('/file-differences');
  await page.getByRole('button', { name: /Synthetic series/ }).click();
  const dialog = page.getByRole('dialog');
  await expect(
    dialog.getByText('Additional audio', { exact: true }),
  ).toHaveCount(0);
  await dialog.getByText('Stream details', { exact: true }).first().click();
  await expect(
    dialog.locator('details[open]').getByText('AAC', { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText('Portuguese (Brazil)', { exact: true }).first(),
  ).toBeVisible();
  await expect(
    page.getByText('Portuguese', { exact: true }).first(),
  ).toBeVisible();
  await expect(
    page.getByText('Unknown language', { exact: true }).first(),
  ).toBeVisible();
  await page.goto('/system');
  await page.getByLabel('Show language flags', { exact: true }).check();
  await page.goto('/file-differences');
  await page.getByRole('button', { name: /Synthetic series/ }).click();
  const flag = page
    .getByRole('button', { name: 'Portuguese (Brazil)', exact: true })
    .first();
  await expect(flag).toHaveText('🇧🇷');
  await flag.focus();
  await expect(flag.locator('..').getByRole('tooltip')).toBeVisible();
  await flag.blur();
  if (info.project.use.hasTouch) await flag.tap();
  else await flag.hover();
  await expect(flag.locator('..').getByRole('tooltip')).toBeVisible();
  await expect(
    page.getByText('Portuguese', { exact: true }).first(),
  ).toBeVisible();
});
