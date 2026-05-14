import { test, expect } from '@playwright/test';
import {
	registerViaUI,
	loginViaUI,
	clearFirebaseUsers,
	signInTestUser
} from './helpers';

const RUN_ID = Date.now().toString(36);

test.describe.serial('Cruise Lifecycle', () => {
	const adminEmail = `cl-admin-${RUN_ID}@test.local`;
	const adminPassword = 'TestPass123!';
	const adminName = 'Cruise Admin';

	const orgName = `Sail ${RUN_ID}`;
	const orgSlug = `sail-${RUN_ID}`;
	const orgTripName = `OrgTrip ${RUN_ID}`;
	const personalTripName = `Personal ${RUN_ID}`;

	let orgCruiseId = 0;
	let personalCruiseId = 0;

	test.beforeAll(async () => {
		await clearFirebaseUsers();
	});

	// --- helper to switch to org context ---
	async function selectOrg(page: import('@playwright/test').Page) {
		await page.locator('nav button').first().click();
		await page.getByText(orgName).click();
		await page.waitForURL('/');
	}

	async function selectPersonal(page: import('@playwright/test').Page) {
		await page.locator('nav button').first().click();
		await page.getByRole('button', { name: 'Osobisty' }).click();
		await page.waitForURL('/');
	}

	// ===== ORG CONTEXT TESTS =====

	test('admin registers and creates org', async ({ page }) => {
		await registerViaUI(page, adminName, adminEmail, adminPassword);

		await page.goto('/orgs');
		await page.getByRole('button', { name: 'Nowy klub' }).click();
		await page.getByLabel('Nazwa *').fill(orgName);
		await page.getByLabel('Slug *').fill(orgSlug);
		await page.getByRole('button', { name: 'Utwórz' }).click();
		await page.waitForURL('/', { timeout: 10_000 });
		await expect(page.getByText(orgName)).toBeVisible();
	});

	test('org: create planned trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await selectOrg(page);

		await page.getByRole('link', { name: 'Rejsy' }).click();
		await page.waitForURL('/cruises');

		await page.getByRole('link', { name: '+ Nowy rejs' }).click();
		await page.waitForURL('/cruises/new');

		await page.getByLabel('Planowany').check();
		await page.getByLabel('Nazwa rejsu *').fill(orgTripName);
		await page.getByLabel('Rok').fill('2026');
		await page.getByLabel('Port wyjścia').fill('Gdańsk');
		await page.getByLabel('Port docelowy').fill('Visby');

		await page.getByRole('button', { name: 'Utwórz rejs' }).click();
		await page.waitForURL(/\/cruises\/\d+/, { timeout: 10_000 });
		orgCruiseId = Number(page.url().split('/').pop());

		await expect(page.getByText('Planowany')).toBeVisible({ timeout: 10_000 });
		await expect(page.getByText(orgTripName)).toBeVisible();
	});

	test('org: trip visible in Planowane tab', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await selectOrg(page);

		await page.getByRole('link', { name: 'Rejsy' }).click();
		await page.waitForURL('/cruises');

		await page.getByRole('button', { name: 'Planowane' }).click();
		await expect(page.getByText(orgTripName)).toBeVisible();

		await page.getByRole('button', { name: 'Zrealizowane' }).click();
		await expect(page.getByText('Brak rejsów')).toBeVisible();
	});

	test('org: complete trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await selectOrg(page);

		await page.goto(`/cruises/${orgCruiseId}`);
		page.on('dialog', (d) => d.accept());
		await page.getByRole('button', { name: 'Zrealizuj' }).click();

		await expect(page.getByText('Zrealizowany')).toBeVisible({ timeout: 10_000 });
		await expect(page.getByRole('button', { name: 'Przywróć' })).toBeVisible();
	});

	test('org: add nav stats to completed voyage', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await selectOrg(page);

		await page.goto(`/cruises/${orgCruiseId}/edit`);

		await page.getByLabel('Godziny łącznie').fill('48');
		await page.getByLabel('Mile').fill('120');
		await page.getByLabel('Dni').fill('5');

		await page.getByRole('button', { name: 'Zapisz zmiany' }).click();
		await page.waitForURL(`/cruises/${orgCruiseId}`, { timeout: 10_000 });

		await expect(page.getByText('48')).toBeVisible();
		await expect(page.getByText('120')).toBeVisible();
	});

	test('org: voyage in Zrealizowane tab', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await selectOrg(page);

		await page.getByRole('link', { name: 'Rejsy' }).click();
		await page.waitForURL('/cruises');

		await page.getByRole('button', { name: 'Zrealizowane' }).click();
		await expect(page.getByText(orgTripName)).toBeVisible();

		await page.getByRole('button', { name: 'Planowane' }).click();
		await expect(page.getByText('Brak rejsów')).toBeVisible();
	});

	test('org: reopen voyage', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await selectOrg(page);

		await page.goto(`/cruises/${orgCruiseId}`);
		page.on('dialog', (d) => d.accept());
		await page.getByRole('button', { name: 'Przywróć' }).click();

		await expect(page.getByText('Planowany')).toBeVisible({ timeout: 10_000 });
	});

	test('org: cancel trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await selectOrg(page);

		await page.goto(`/cruises/${orgCruiseId}`);
		page.on('dialog', (d) => d.accept());
		await page.getByRole('button', { name: 'Anuluj' }).click();

		await expect(page.getByText('Anulowany')).toBeVisible({ timeout: 10_000 });
	});

	// ===== PERSONAL CONTEXT TESTS =====

	test('personal: create planned trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await selectPersonal(page);

		await page.getByRole('link', { name: 'Rejsy' }).click();
		await page.waitForURL('/cruises');

		await page.getByRole('link', { name: '+ Nowy rejs' }).click();
		await page.waitForURL('/cruises/new');

		await page.getByLabel('Planowany').check();
		await page.getByLabel('Nazwa rejsu *').fill(personalTripName);
		await page.getByLabel('Port wyjścia').fill('Split');
		await page.getByLabel('Port docelowy').fill('Dubrovnik');

		await page.getByRole('button', { name: 'Utwórz rejs' }).click();
		await page.waitForURL(/\/cruises\/\d+/, { timeout: 10_000 });
		personalCruiseId = Number(page.url().split('/').pop());

		await expect(page.getByText('Planowany')).toBeVisible();
	});

	test('personal: enrollment on planned trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await selectPersonal(page);

		await page.goto(`/cruises/${personalCruiseId}`);
		await page.getByRole('button', { name: 'Włącz zapisy' }).click();
		await expect(page.getByRole('button', { name: 'Kopiuj link' })).toBeVisible({ timeout: 10_000 });
	});

	test('personal: complete trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await selectPersonal(page);

		await page.goto(`/cruises/${personalCruiseId}`);
		page.on('dialog', (d) => d.accept());
		await page.getByRole('button', { name: 'Zrealizuj' }).click();

		await expect(page.getByText('Zrealizowany')).toBeVisible({ timeout: 10_000 });
	});

	test('personal: enrollment rejected on completed', async () => {
		const auth = await signInTestUser(adminEmail, adminPassword);
		const res = await fetch(
			`${process.env.API_BASE_URL || 'http://localhost:5173/api'}/cruises/${personalCruiseId}`,
			{
				headers: { Authorization: `Bearer ${auth.idToken}` }
			}
		);
		const cruise = (await res.json()) as { enroll_token: string };
		if (cruise.enroll_token) {
			const enrollRes = await fetch(
				`${process.env.API_BASE_URL || 'http://localhost:5173/api'}/enroll/${cruise.enroll_token}`,
				{
					method: 'POST',
					headers: {
						'Content-Type': 'application/json',
						Authorization: `Bearer ${auth.idToken}`
					},
					body: JSON.stringify({ note: 'test' })
				}
			);
			expect(enrollRes.ok).toBeFalsy();
		}
	});

	test('personal: voyage in Zrealizowane tab', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await selectPersonal(page);

		await page.getByRole('link', { name: 'Rejsy' }).click();
		await page.waitForURL('/cruises');

		await page.getByRole('button', { name: 'Zrealizowane' }).click();
		await expect(page.getByText(personalTripName)).toBeVisible();
	});
});
