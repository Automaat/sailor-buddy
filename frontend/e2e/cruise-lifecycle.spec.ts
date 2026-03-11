import { test, expect } from '@playwright/test';
import {
	registerViaUI,
	loginViaUI,
	clearFirebaseUsers,
	signInTestUser,
	apiRequest
} from './helpers';

const RUN_ID = Date.now().toString(36);

test.describe.serial('Cruise Lifecycle', () => {
	const adminEmail = `cl-admin-${RUN_ID}@test.local`;
	const adminPassword = 'TestPass123!';
	const adminName = 'Cruise Admin';

	const memberEmail = `cl-member-${RUN_ID}@test.local`;
	const memberPassword = 'TestPass123!';
	const memberName = 'Cruise Member';

	const orgName = `Sail ${RUN_ID}`;
	const orgSlug = `sail-${RUN_ID}`;
	const tripName = `Trip ${RUN_ID}`;
	const personalTripName = `Personal ${RUN_ID}`;

	let cruiseId = 0;
	let personalCruiseId = 0;
	let enrollToken = '';

	test.beforeAll(async () => {
		await clearFirebaseUsers();
	});

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

	test('admin creates planned trip in org', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		// select org
		await page.locator('nav button').first().click();
		await page.getByText(orgName).click();
		await page.waitForURL('/');

		await page.getByRole('link', { name: 'Rejsy' }).click();
		await page.waitForURL('/cruises');

		await page.getByRole('link', { name: '+ Nowy rejs' }).click();
		await page.waitForURL('/cruises/new');

		// select planned status
		await page.getByLabel('Planowany').check();

		await page.getByLabel('Nazwa rejsu *').fill(tripName);
		await page.getByLabel('Rok').fill('2026');
		await page.getByLabel('Port wyjścia').fill('Gdańsk');
		await page.getByLabel('Port docelowy').fill('Visby');
		await page.getByLabel('Maks. załoga').fill('8');

		// capture API response for debugging
		const responsePromise = page.waitForResponse((r) => r.url().includes('/cruises') && r.request().method() === 'POST');
		await page.getByRole('button', { name: 'Utwórz rejs' }).click();
		const response = await responsePromise;
		console.log('Create cruise URL:', response.url());
		console.log('Create cruise request body:', response.request().postData());
		console.log('Create cruise response:', response.status(), await response.text());
		expect(response.ok()).toBeTruthy();

		// should redirect to cruise detail
		await page.waitForURL(/\/cruises\/\d+/, { timeout: 10_000 });
		cruiseId = Number(page.url().split('/').pop());

		// verify planned badge visible
		await expect(page.getByText('Planowany')).toBeVisible();
		// nav stats should not be shown (no hours card with value)
		await expect(page.getByText(tripName)).toBeVisible();
	});

	test('admin sees trip in Planowane tab', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		// select org
		await page.locator('nav button').first().click();
		await page.getByText(orgName).click();
		await page.waitForURL('/');

		await page.getByRole('link', { name: 'Rejsy' }).click();
		await page.waitForURL('/cruises');

		// click Planowane tab
		await page.getByRole('button', { name: 'Planowane' }).click();
		await expect(page.getByText(tripName)).toBeVisible();

		// click Zrealizowane tab - trip should not be there
		await page.getByRole('button', { name: 'Zrealizowane' }).click();
		await expect(page.getByText('Brak rejsów')).toBeVisible();
	});

	test('admin generates enrollment link', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		// select org
		await page.locator('nav button').first().click();
		await page.getByText(orgName).click();
		await page.waitForURL('/');

		await page.goto(`/cruises/${cruiseId}`);
		await page.getByRole('button', { name: 'Włącz zapisy' }).click();
		await expect(page.getByRole('button', { name: 'Kopiuj link' })).toBeVisible({ timeout: 10_000 });

		// get token via API
		const auth = await signInTestUser(adminEmail, adminPassword);
		const cruise = (await apiRequest(auth.idToken, 'GET', `/orgs/${orgSlug}/cruises/${cruiseId}`)) as {
			enroll_token: string;
		};
		expect(cruise.enroll_token).toBeTruthy();
		enrollToken = cruise.enroll_token;
	});

	test('member registers and enrolls in trip', async ({ page }) => {
		await registerViaUI(page, memberName, memberEmail, memberPassword);

		await page.goto(`/enroll/${enrollToken}`);
		await expect(page.getByText(tripName)).toBeVisible({ timeout: 10_000 });

		await page.getByRole('button', { name: 'Zapisz się' }).click();
		await expect(page.getByText('Zapisano')).toBeVisible({ timeout: 10_000 });
	});

	test('admin creates crew member and assigns to trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		// select org
		await page.locator('nav button').first().click();
		await page.getByText(orgName).click();
		await page.waitForURL('/');

		// create crew member
		await page.getByRole('link', { name: 'Załoga' }).click();
		await page.waitForURL('/crew');
		await page.getByRole('button', { name: '+ Dodaj załoganta' }).click();
		await page.getByLabel('Imię i nazwisko *').fill('Bosman Janek');
		await page.getByRole('button', { name: 'Dodaj' }).click();
		await expect(page.getByText('Bosman Janek')).toBeVisible();

		// assign to cruise
		await page.goto(`/cruises/${cruiseId}`);
		await page.locator('#assign-crew').selectOption({ label: 'Bosman Janek' });
		await page.locator('#assign-role').fill('Sternik');
		await page.getByRole('button', { name: 'Dodaj' }).click();

		await expect(page.getByText('Bosman Janek')).toBeVisible({ timeout: 10_000 });
		await expect(page.getByText('Sternik')).toBeVisible();
	});

	test('admin completes trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		// select org
		await page.locator('nav button').first().click();
		await page.getByText(orgName).click();
		await page.waitForURL('/');

		await page.goto(`/cruises/${cruiseId}`);

		// accept confirm dialog
		page.on('dialog', (d) => d.accept());
		await page.getByRole('button', { name: 'Zrealizuj' }).click();

		// should show completed badge
		await expect(page.getByText('Zrealizowany')).toBeVisible({ timeout: 10_000 });
		// Zrealizuj button should be gone, Przywróć should appear
		await expect(page.getByRole('button', { name: 'Przywróć' })).toBeVisible();
	});

	test('enrollment rejected on completed cruise', async () => {
		// try to enroll via API - should fail
		const auth = await signInTestUser(memberEmail, memberPassword);
		const res = await fetch(
			`${process.env.API_BASE_URL || 'http://localhost:5173/api'}/enroll/${enrollToken}`,
			{
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${auth.idToken}`
				},
				body: JSON.stringify({ note: 'test' })
			}
		);
		// enrollment should be rejected (conflict or not found since token may be cleared)
		expect(res.ok).toBeFalsy();
	});

	test('admin adds nav stats to completed voyage', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		// select org
		await page.locator('nav button').first().click();
		await page.getByText(orgName).click();
		await page.waitForURL('/');

		await page.goto(`/cruises/${cruiseId}/edit`);

		await page.getByLabel('Godziny łącznie').fill('48');
		await page.getByLabel('Mile').fill('120');
		await page.getByLabel('Dni').fill('5');

		await page.getByRole('button', { name: 'Zapisz zmiany' }).click();
		await page.waitForURL(`/cruises/${cruiseId}`, { timeout: 10_000 });

		await expect(page.getByText('48')).toBeVisible();
		await expect(page.getByText('120')).toBeVisible();
	});

	test('completed voyage in Zrealizowane tab', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		// select org
		await page.locator('nav button').first().click();
		await page.getByText(orgName).click();
		await page.waitForURL('/');

		await page.getByRole('link', { name: 'Rejsy' }).click();
		await page.waitForURL('/cruises');

		await page.getByRole('button', { name: 'Zrealizowane' }).click();
		await expect(page.getByText(tripName)).toBeVisible();

		// not in Planowane
		await page.getByRole('button', { name: 'Planowane' }).click();
		await expect(page.getByText('Brak rejsów')).toBeVisible();
	});

	test('admin reopens voyage', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		// select org
		await page.locator('nav button').first().click();
		await page.getByText(orgName).click();
		await page.waitForURL('/');

		await page.goto(`/cruises/${cruiseId}`);

		page.on('dialog', (d) => d.accept());
		await page.getByRole('button', { name: 'Przywróć' }).click();

		await expect(page.getByText('Planowany')).toBeVisible({ timeout: 10_000 });
	});

	test('admin cancels trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		// select org
		await page.locator('nav button').first().click();
		await page.getByText(orgName).click();
		await page.waitForURL('/');

		await page.goto(`/cruises/${cruiseId}`);

		page.on('dialog', (d) => d.accept());
		await page.getByRole('button', { name: 'Anuluj' }).click();

		await expect(page.getByText('Anulowany')).toBeVisible({ timeout: 10_000 });
	});

	test('personal user creates planned trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		// ensure personal mode
		await page.locator('nav button').first().click();
		await page.getByRole('button', { name: '👤 Osobisty' }).click();
		await page.waitForURL('/');

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

	test('personal user completes trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		// ensure personal mode
		await page.locator('nav button').first().click();
		await page.getByRole('button', { name: '👤 Osobisty' }).click();
		await page.waitForURL('/');

		await page.goto(`/cruises/${personalCruiseId}`);

		page.on('dialog', (d) => d.accept());
		await page.getByRole('button', { name: 'Zrealizuj' }).click();

		await expect(page.getByText('Zrealizowany')).toBeVisible({ timeout: 10_000 });

		// check it shows in voyages tab
		await page.getByRole('link', { name: 'Rejsy' }).click();
		await page.waitForURL('/cruises');

		await page.getByRole('button', { name: 'Zrealizowane' }).click();
		await expect(page.getByText(personalTripName)).toBeVisible();
	});
});
