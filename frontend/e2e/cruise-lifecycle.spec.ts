import { test, expect, type Page } from '@playwright/test';
import { registerViaUI, loginViaUI, clearFirebaseUsers } from './helpers';

const RUN_ID = Date.now().toString(36);

test.describe.serial('Trip/Voyage Lifecycle', () => {
	const adminEmail = `cl-admin-${RUN_ID}@test.local`;
	const adminPassword = 'TestPass123!';
	const adminName = 'Cruise Admin';

	const soloEmail = `cl-solo-${RUN_ID}@test.local`;
	const soloPassword = 'TestPass123!';
	const soloName = 'Solo Sailor';

	const orgName = `Sail ${RUN_ID}`;
	const orgSlug = `sail-${RUN_ID}`;
	const orgTripName = `OrgTrip ${RUN_ID}`;
	const personalTripName = `Personal ${RUN_ID}`;

	let orgTripId = 0;
	let personalTripId = 0;

	test.beforeAll(async () => {
		await clearFirebaseUsers();
	});

	// A single-org user has their org auto-selected on login — it shows in the nav.
	async function expectOrgActive(page: Page) {
		await expect(page.locator('nav').getByText(orgName)).toBeVisible();
	}

	test('admin registers and creates org', async ({ page }) => {
		await registerViaUI(page, adminName, adminEmail, adminPassword);

		await page.goto('/orgs');
		await page.getByRole('button', { name: 'Nowy klub' }).click();
		await page.getByLabel('Nazwa *').fill(orgName);
		await page.getByLabel('Slug *').fill(orgSlug);
		await page.getByRole('button', { name: 'Utwórz' }).click();
		await page.waitForURL('/', { timeout: 10_000 });
		await expectOrgActive(page);
	});

	test('org: create planned trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.getByRole('link', { name: 'Planowane' }).click();
		await page.waitForURL('/trips');

		await page.getByRole('link', { name: '+ Zaplanuj rejs' }).click();
		await page.waitForURL('/trips/new');

		await page.getByLabel('Nazwa rejsu *').fill(orgTripName);
		await page.getByLabel('Port wyjścia').fill('Gdańsk');
		await page.getByLabel('Port docelowy').fill('Visby');

		await page.getByRole('button', { name: 'Zaplanuj rejs' }).click();
		await page.waitForURL(/\/trips\/\d+/, { timeout: 10_000 });
		orgTripId = Number(page.url().split('/').pop());

		await expect(page.getByText('Planowany')).toBeVisible({ timeout: 10_000 });
		await expect(page.getByRole('heading', { name: orgTripName })).toBeVisible();
	});

	test('org: trip visible in planned list', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.getByRole('link', { name: 'Planowane' }).click();
		await page.waitForURL('/trips');
		await expect(page.getByText(orgTripName)).toBeVisible();
	});

	test('org: complete trip into voyage', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.goto(`/trips/${orgTripId}`);
		page.on('dialog', (d) => d.accept());
		await page.getByRole('button', { name: 'Zrealizuj' }).click();

		// Modal opens — fill stats and submit. Rok/Dni/Godziny łącznie are computed.
		await page.getByLabel('Godziny żagli').fill('30');
		await page.getByLabel('Godziny silnika').fill('18');
		await page.getByLabel('Mile').fill('120');
		await page.getByRole('button', { name: 'Zrealizuj rejs' }).click();

		await page.waitForURL(/\/voyages\/\d+/, { timeout: 15_000 });
		await expect(page.getByRole('heading', { name: orgTripName })).toBeVisible();
		await expect(page.getByText('120')).toBeVisible();
	});

	test('org: voyage in completed list', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.getByRole('link', { name: 'Zrealizowane' }).click();
		await page.waitForURL('/voyages');
		await expect(page.getByText(orgTripName)).toBeVisible();
	});

	test('solo user registers without an org', async ({ page }) => {
		await registerViaUI(page, soloName, soloEmail, soloPassword);

		// no org -> no club-only nav items
		await expect(page.getByRole('link', { name: 'Wydarzenia' })).toHaveCount(0);
	});

	test('personal: create planned trip', async ({ page }) => {
		await loginViaUI(page, soloEmail, soloPassword);

		await page.getByRole('link', { name: 'Planowane' }).click();
		await page.waitForURL('/trips');

		await page.getByRole('link', { name: '+ Zaplanuj rejs' }).click();
		await page.waitForURL('/trips/new');

		await page.getByLabel('Nazwa rejsu *').fill(personalTripName);
		await page.getByLabel('Port wyjścia').fill('Split');
		await page.getByLabel('Port docelowy').fill('Dubrovnik');

		await page.getByRole('button', { name: 'Zaplanuj rejs' }).click();
		await page.waitForURL(/\/trips\/\d+/, { timeout: 10_000 });
		personalTripId = Number(page.url().split('/').pop());

		await expect(page.getByText('Planowany')).toBeVisible();
	});

	test('personal: enrollment toggle on planned trip', async ({ page }) => {
		await loginViaUI(page, soloEmail, soloPassword);

		await page.goto(`/trips/${personalTripId}`);
		await page.getByRole('button', { name: 'Włącz zapisy' }).click();
		await expect(page.getByRole('button', { name: 'Kopiuj link' })).toBeVisible({ timeout: 10_000 });
	});

	test('personal: cancel trip', async ({ page }) => {
		await loginViaUI(page, soloEmail, soloPassword);

		await page.goto(`/trips/${personalTripId}`);
		page.on('dialog', (d) => d.accept());
		await page.getByRole('button', { name: 'Anuluj' }).click();

		await expect(page.getByText('Anulowany')).toBeVisible({ timeout: 10_000 });
	});
});
