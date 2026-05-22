import { test, expect } from '@playwright/test';
import { registerViaUI, loginViaUI, clearFirebaseUsers } from './helpers';

// Exercises the club cruise/trip/voyage lifecycle as the admin: create a
// cruise, plan a trip, complete it into a voyage. The first registered user
// becomes the club admin. Assumes a freshly migrated database.

const RUN_ID = Date.now().toString(36);

test.describe.serial('Cruise / trip / voyage lifecycle', () => {
	const adminEmail = `cl-admin-${RUN_ID}@test.local`;
	const adminPassword = 'TestPass123!';
	const adminName = 'Cruise Admin';

	const cruiseName = `Cruise ${RUN_ID}`;
	const tripName = `Trip ${RUN_ID}`;

	let cruiseId = 0;
	let tripId = 0;

	test.beforeAll(async () => {
		await clearFirebaseUsers();
	});

	test('admin registers and is the club admin', async ({ page }) => {
		await registerViaUI(page, adminName, adminEmail, adminPassword);
		// the members nav only shows for admins
		await expect(page.getByRole('link', { name: 'Członkowie' })).toHaveCount(1);
	});

	test('admin creates a cruise', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		await page.getByRole('link', { name: 'Wydarzenia' }).click();
		await page.waitForURL('/cruises');

		await page.getByRole('link', { name: '+ Nowe wydarzenie' }).click();
		await page.waitForURL('/cruises/new');

		await page.getByLabel('Nazwa wydarzenia *').fill(cruiseName);
		await page.getByRole('button', { name: 'Utwórz wydarzenie' }).click();

		await page.waitForURL(/\/cruises\/\d+/, { timeout: 10_000 });
		cruiseId = Number(page.url().split('/').pop());
		await expect(page.getByRole('heading', { name: cruiseName })).toBeVisible();
	});

	test('admin creates a planned trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		await page.goto('/trips/new');
		await page.getByLabel('Nazwa rejsu *').fill(tripName);
		await page.getByLabel('Port wyjścia').fill('Gdańsk');
		await page.getByLabel('Port docelowy').fill('Visby');
		await page.getByRole('button', { name: 'Zaplanuj rejs' }).click();

		await page.waitForURL(/\/trips\/\d+/, { timeout: 10_000 });
		tripId = Number(page.url().split('/').pop());
		await expect(page.getByText('Planowany')).toBeVisible({ timeout: 10_000 });
		await expect(page.getByRole('heading', { name: tripName })).toBeVisible();
	});

	test('trip appears in the planned list', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		await page.getByRole('link', { name: 'Planowane' }).click();
		await page.waitForURL('/trips');
		await expect(page.getByText(tripName)).toBeVisible();
	});

	test('admin completes the trip into a voyage', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		await page.goto(`/trips/${tripId}`);
		page.on('dialog', (d) => d.accept());
		await page.getByRole('button', { name: 'Zrealizuj' }).click();

		await page.getByLabel('Godziny żagli').fill('30');
		await page.getByLabel('Godziny silnika').fill('18');
		await page.getByLabel('Mile').fill('120');
		await page.getByRole('button', { name: 'Zrealizuj rejs' }).click();

		await page.waitForURL(/\/voyages\/\d+/, { timeout: 15_000 });
		await expect(page.getByRole('heading', { name: tripName })).toBeVisible();
		await expect(page.getByText('120')).toBeVisible();
	});

	test('voyage appears in the completed list', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		await page.getByRole('link', { name: 'Zrealizowane' }).click();
		await page.waitForURL('/voyages');
		await expect(page.getByText(tripName)).toBeVisible();
	});

	test('admin enables enrollment on the cruise', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		await page.goto(`/cruises/${cruiseId}`);
		await page.getByRole('button', { name: 'Włącz zapisy' }).click();
		await expect(page.getByRole('button', { name: 'Kopiuj link' })).toBeVisible({
			timeout: 10_000
		});
	});
});
