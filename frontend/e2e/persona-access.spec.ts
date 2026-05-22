import { test, expect } from '@playwright/test';
import { createTestUser, registerViaUI, loginViaUI, clearFirebaseUsers, apiRequest } from './helpers';

// Single-club role model. The first account ever provisioned auto-becomes the
// club admin; every account after is a regular member. This is the source of
// truth for the role feature — it covers the persona matrix (admin vs member)
// and the admin's cruise/trip/voyage lifecycle in one serial run so the
// "first user" admin assumption stays deterministic.
//
// Runs against a freshly migrated database (no seed) — see .github/workflows/e2e.yml.

const RUN_ID = Date.now().toString(36);
const PASSWORD = 'TestPass123!';
const adminEmail = `pa-admin-${RUN_ID}@test.local`;
const adminName = 'Persona Admin';
const memberEmail = `pa-member-${RUN_ID}@test.local`;

const API_BASE = process.env.API_BASE_URL || 'http://localhost:5173/api';

const cruiseName = `Cruise ${RUN_ID}`;
const tripName = `Trip ${RUN_ID}`;
let tripId = 0;
let cruiseId = 0;

// mutationStatus issues an authenticated mutating request and returns the HTTP
// status, so a member can be asserted forbidden (403).
async function mutationStatus(
	token: string,
	method: string,
	path: string,
	body?: unknown
): Promise<number> {
	const res = await fetch(`${API_BASE}${path}`, {
		method,
		headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
		body: body ? JSON.stringify(body) : undefined
	});
	return res.status;
}

test.describe.serial('Single-club persona matrix', () => {
	let memberToken = '';

	test.beforeAll(async () => {
		await clearFirebaseUsers();
	});

	test('setup: first user becomes admin, second becomes member', async ({ page }) => {
		// Register the admin through the UI first so it is the first account
		// the backend provisions — that account is auto-promoted to admin.
		await registerViaUI(page, adminName, adminEmail, PASSWORD);
		await expect(page.getByRole('link', { name: 'Członkowie' })).toHaveCount(1);

		const member = await createTestUser(memberEmail, PASSWORD, 'Persona Member');
		memberToken = member.idToken;
		const memberMe = (await apiRequest(member.idToken, 'GET', '/auth/me')) as { role: string };
		expect(memberMe.role).toBe('member');
	});

	test('member: no members nav, cruises read-only, mutations 403', async ({ page }) => {
		await loginViaUI(page, memberEmail, PASSWORD);

		await expect(page.getByRole('link', { name: 'Członkowie' })).toHaveCount(0);

		await page.goto('/cruises');
		await expect(page.getByRole('link', { name: '+ Nowe wydarzenie' })).toHaveCount(0);

		// the members page redirects a non-admin away
		await page.goto('/members');
		await page.waitForURL('/', { timeout: 10_000 });

		expect(await mutationStatus(memberToken, 'POST', '/cruises', { name: 'X' })).toBe(403);
		expect(await mutationStatus(memberToken, 'POST', '/trips', { name: 'X' })).toBe(403);
		expect(await mutationStatus(memberToken, 'POST', '/yachts', { name: 'X' })).toBe(403);
		expect(await mutationStatus(memberToken, 'POST', '/crew', { full_name: 'X' })).toBe(403);
	});

	test('admin: sees the members roster', async ({ page }) => {
		await loginViaUI(page, adminEmail, PASSWORD);
		await page.goto('/members');
		await expect(page.getByRole('heading', { name: 'Członkowie' })).toBeVisible();
		await expect(page.getByText(memberEmail)).toBeVisible();
	});

	test('admin: creates a cruise', async ({ page }) => {
		await loginViaUI(page, adminEmail, PASSWORD);

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

	test('admin: creates a planned trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, PASSWORD);

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

	test('admin: completes the trip into a voyage', async ({ page }) => {
		await loginViaUI(page, adminEmail, PASSWORD);

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
		await loginViaUI(page, adminEmail, PASSWORD);
		await page.getByRole('link', { name: 'Zrealizowane' }).click();
		await page.waitForURL('/voyages');
		await expect(page.getByText(tripName)).toBeVisible();
	});

	test('member: sees the club data read-only', async ({ page }) => {
		await loginViaUI(page, memberEmail, PASSWORD);

		await page.goto('/cruises');
		await expect(page.getByText(cruiseName)).toBeVisible();
		await expect(page.getByRole('link', { name: '+ Nowe wydarzenie' })).toHaveCount(0);

		await page.goto('/voyages');
		await expect(page.getByText(tripName)).toBeVisible();
		await expect(page.getByRole('link', { name: '+ Wpisz rejs' })).toHaveCount(0);
	});
});
