import { test, expect } from '@playwright/test';
import { createTestUser, loginViaUI, clearFirebaseUsers, apiRequest } from './helpers';

// Persona matrix for the single-club role model. There are two personas:
// `admin` (the first account ever provisioned — auto-promoted to club admin)
// and `member` (every account after). This spec is the source of truth for
// the role feature: admins manage club data, members get read-only access.
//
// Assumes a freshly migrated database with no pre-existing admin.

const RUN_ID = Date.now().toString(36);
const PASSWORD = 'TestPass123!';
const adminEmail = `pa-admin-${RUN_ID}@test.local`;
const memberEmail = `pa-member-${RUN_ID}@test.local`;

const API_BASE = process.env.API_BASE_URL || 'http://localhost:5173/api';

// mutationStatus issues an authenticated mutating request and returns the HTTP
// status, so the test can assert a member is forbidden (403).
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

test.describe.serial('Persona access matrix', () => {
	let memberToken = '';

	test.beforeAll(async () => {
		await clearFirebaseUsers();
	});

	test('setup: first user becomes admin, second becomes member', async () => {
		// The first account provisioned becomes the club admin — provision it
		// first by making one authenticated request, then the member.
		const admin = await createTestUser(adminEmail, PASSWORD, 'Persona Admin');
		const adminMe = (await apiRequest(admin.idToken, 'GET', '/auth/me')) as { role: string };
		expect(adminMe.role).toBe('admin');

		const member = await createTestUser(memberEmail, PASSWORD, 'Persona Member');
		memberToken = member.idToken;
		const memberMe = (await apiRequest(member.idToken, 'GET', '/auth/me')) as { role: string };
		expect(memberMe.role).toBe('member');
	});

	test('admin: sees the members nav and can manage cruises', async ({ page }) => {
		await loginViaUI(page, adminEmail, PASSWORD);

		await expect(page.getByRole('link', { name: 'Członkowie' })).toHaveCount(1);

		await page.goto('/cruises');
		await expect(page.getByRole('link', { name: '+ Nowe wydarzenie' })).toHaveCount(1);

		await page.goto('/members');
		await expect(page.getByRole('heading', { name: 'Członkowie' })).toBeVisible();
	});

	test('member: no members nav, cruises are read-only', async ({ page }) => {
		await loginViaUI(page, memberEmail, PASSWORD);

		await expect(page.getByRole('link', { name: 'Członkowie' })).toHaveCount(0);

		await page.goto('/cruises');
		await expect(page.getByRole('link', { name: '+ Nowe wydarzenie' })).toHaveCount(0);

		// the members page redirects a non-admin away
		await page.goto('/members');
		await page.waitForURL('/', { timeout: 10_000 });
	});

	test('member: club-data mutations are rejected with 403', async () => {
		expect(await mutationStatus(memberToken, 'POST', '/cruises', { name: 'X' })).toBe(403);
		expect(await mutationStatus(memberToken, 'POST', '/trips', { name: 'X' })).toBe(403);
		expect(await mutationStatus(memberToken, 'POST', '/yachts', { name: 'X' })).toBe(403);
		expect(await mutationStatus(memberToken, 'POST', '/crew', { full_name: 'X' })).toBe(403);
	});
});
