import { test, expect, type Page } from '@playwright/test';
import { registerViaUI, loginViaUI, clearFirebaseUsers } from './helpers';

const RUN_ID = Date.now().toString(36);

// Exercises the org-scoped crew assignment and voyage opinion routes through
// the UI: assigning crew to an org trip, completing it into a voyage, and
// generating an opinion document for the repointed crew member.
test.describe.serial('Org crew assignment and opinions', () => {
	const adminEmail = `oco-admin-${RUN_ID}@test.local`;
	const adminPassword = 'TestPass123!';
	const adminName = 'Opinion Admin';

	const orgName = `Opinion Club ${RUN_ID}`;
	const orgSlug = `opinion-club-${RUN_ID}`;
	const crewName = `Crew ${RUN_ID}`;
	const tripName = `OrgVoyage ${RUN_ID}`;

	let tripId = 0;

	test.beforeAll(async () => {
		await clearFirebaseUsers();
	});

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

	test('admin creates an org crew member', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.goto('/crew');
		await page.getByRole('button', { name: '+ Dodaj załoganta' }).click();
		await page.getByLabel('Imię i nazwisko *').fill(crewName);
		await page.getByRole('button', { name: 'Dodaj' }).click();

		await expect(page.getByText(crewName)).toBeVisible();
	});

	test('admin creates an org trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.getByRole('link', { name: 'Planowane' }).click();
		await page.waitForURL('/trips');
		await page.getByRole('link', { name: '+ Zaplanuj rejs' }).click();
		await page.waitForURL('/trips/new');

		await page.getByLabel('Nazwa rejsu *').fill(tripName);
		await page.getByLabel('Port wyjścia').fill('Gdynia');
		await page.getByLabel('Port docelowy').fill('Karlskrona');
		await page.getByRole('button', { name: 'Zaplanuj rejs' }).click();

		await page.waitForURL(/\/trips\/\d+/, { timeout: 10_000 });
		tripId = Number(page.url().split('/').pop());
		await expect(page.getByRole('heading', { name: tripName })).toBeVisible();
	});

	test('admin assigns crew to the org trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.goto(`/trips/${tripId}`);

		await page.locator('#assign-crew').selectOption({ label: crewName });
		await page.locator('#assign-role').fill('Sternik');
		await page.getByRole('button', { name: 'Dodaj' }).click();

		// crew row renders the member name (in a span, distinct from the
		// select option) plus the role badge
		await expect(page.locator('span.font-medium', { hasText: crewName })).toBeVisible({
			timeout: 10_000
		});
		await expect(page.getByText('Sternik')).toBeVisible();
	});

	test('admin completes the trip into a voyage', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.goto(`/trips/${tripId}`);
		page.on('dialog', (d) => d.accept());
		await page.getByRole('button', { name: 'Zrealizuj' }).click();

		await page.getByLabel('Godziny żagli').fill('24');
		await page.getByLabel('Godziny silnika').fill('12');
		await page.getByLabel('Mile').fill('90');
		await page.getByRole('button', { name: 'Zrealizuj rejs' }).click();

		await page.waitForURL(/\/voyages\/\d+/, { timeout: 15_000 });
		// crew assignment was repointed from the trip to the new voyage
		await expect(page.locator('span.font-medium', { hasText: crewName })).toBeVisible({
			timeout: 10_000
		});
	});

	test('admin generates a crew opinion for the org voyage', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.getByRole('link', { name: 'Zrealizowane' }).click();
		await page.waitForURL('/voyages');
		await page.getByText(tripName).click();
		await page.waitForURL(/\/voyages\/\d+/);

		await page.locator('#gen-crew').selectOption({ label: crewName });
		await page.locator('#gen-format').selectOption('docx');
		await page.getByRole('button', { name: 'Generuj' }).click();

		// generated opinion row appears with a download button and format badge
		await expect(page.getByRole('button', { name: 'Pobierz' })).toBeVisible({ timeout: 15_000 });
		await expect(page.locator('span.uppercase', { hasText: 'docx' })).toBeVisible();
	});
});
