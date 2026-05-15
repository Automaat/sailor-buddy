import { test, expect, type Page } from '@playwright/test';
import {
	registerViaUI,
	loginViaUI,
	clearFirebaseUsers,
	signInTestUser,
	apiRequest
} from './helpers';

const RUN_ID = Date.now().toString(36);

test.describe.serial('Organization Flow', () => {
	const adminEmail = `admin-${RUN_ID}@test.local`;
	const adminPassword = 'TestPass123!';
	const adminName = 'Admin User';

	const memberEmail = `member-${RUN_ID}@test.local`;
	const memberPassword = 'TestPass123!';
	const memberName = 'Member User';

	const orgName = `Club ${RUN_ID}`;
	const orgSlug = `club-${RUN_ID}`;

	let inviteToken = '';

	test.beforeAll(async () => {
		await clearFirebaseUsers();
	});

	// A single-org user has their org auto-selected on login — it shows in the nav.
	async function expectOrgActive(page: Page) {
		await expect(page.locator('nav').getByText(orgName)).toBeVisible();
	}

	test('admin registers and sees dashboard', async ({ page }) => {
		await registerViaUI(page, adminName, adminEmail, adminPassword);
		await expect(page.getByRole('heading', { name: 'Pulpit' })).toBeVisible();
	});

	test('admin creates organization', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);

		await page.goto('/orgs');
		await expect(page.getByRole('heading', { name: 'Kluby żeglarskie' })).toBeVisible();

		await page.getByRole('button', { name: 'Nowy klub' }).click();

		await page.getByLabel('Nazwa *').fill(orgName);
		await page.getByLabel('Slug *').fill(orgSlug);
		await page.getByLabel('Miasto').fill('Gdańsk');

		await page.getByRole('button', { name: 'Utwórz' }).click();

		// redirect to dashboard with the new org auto-selected
		await page.waitForURL('/', { timeout: 10_000 });
		await expectOrgActive(page);
	});

	test('admin sees org dashboard', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await expect(page.getByRole('heading', { name: 'Pulpit' })).toBeVisible();
	});

	test('admin sees members page', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.getByRole('link', { name: 'Członkowie' }).click();
		await page.waitForURL(`**/orgs/${orgSlug}/members`);

		await expect(page.getByText(adminName)).toBeVisible();
	});

	test('admin creates invite link', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.getByRole('link', { name: 'Członkowie' }).click();
		await page.waitForURL(`**/orgs/${orgSlug}/members`);

		await page.getByRole('button', { name: 'Zaproś' }).click();
		await expect(page.getByText('Nowe zaproszenie')).toBeVisible();

		await page.getByRole('button', { name: 'Utwórz link' }).click();

		await expect(page.getByText('Aktywne zaproszenia')).toBeVisible();
		await expect(page.getByRole('button', { name: 'Kopiuj link' })).toBeVisible();

		// grab token via API for the next test
		const adminAuth = await signInTestUser(adminEmail, adminPassword);
		const invites = (await apiRequest(
			adminAuth.idToken,
			'GET',
			`/orgs/${orgSlug}/invites`
		)) as Array<{ token: string }>;
		expect(invites.length).toBeGreaterThan(0);
		inviteToken = invites[0].token;
	});

	test('member registers and joins org via invite', async ({ page }) => {
		await registerViaUI(page, memberName, memberEmail, memberPassword);

		await page.goto(`/join/${inviteToken}`);

		await expect(page.getByText(orgName)).toBeVisible();
		await expect(page.getByText('Załogant')).toBeVisible();

		await page.getByRole('button', { name: 'Dołącz do klubu' }).click();
		await expect(page.getByText('Dołączono!')).toBeVisible();

		await page.waitForURL('/', { timeout: 10_000 });
	});

	test('member sees org name in nav', async ({ page }) => {
		await loginViaUI(page, memberEmail, memberPassword);
		await expect(page.locator('nav').getByText(orgName)).toBeVisible();
	});

	test('admin creates org yacht', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.getByRole('link', { name: 'Jachty' }).click();
		await page.waitForURL('/yachts');

		await page.getByRole('button', { name: '+ Dodaj jacht' }).click();
		await page.getByLabel('Nazwa *').fill('Orion');
		await page.getByLabel('Typ').fill('Delphia 40');
		await page.getByRole('button', { name: 'Dodaj' }).click();

		await expect(page.getByText('Orion')).toBeVisible();
	});

	test('admin creates org crew member', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.goto('/crew');
		await page.waitForURL('/crew');

		await page.getByRole('button', { name: '+ Dodaj załoganta' }).click();
		await page.getByLabel('Imię i nazwisko *').fill('Jan Kowalski');
		await page.getByRole('button', { name: 'Dodaj' }).click();

		await expect(page.getByText('Jan Kowalski')).toBeVisible();
	});

	test('member sees org resources', async ({ page }) => {
		await loginViaUI(page, memberEmail, memberPassword);
		await expect(page.locator('nav').getByText(orgName)).toBeVisible();

		await page.getByRole('link', { name: 'Jachty' }).click();
		await page.waitForURL('/yachts');
		await expect(page.getByText('Orion')).toBeVisible();

		await page.goto('/crew');
		await page.waitForURL('/crew');
		await expect(page.getByText('Jan Kowalski')).toBeVisible();
	});

	test('admin updates org settings', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.getByRole('link', { name: 'Ustawienia' }).click();
		await page.waitForURL(`**/orgs/${orgSlug}/settings`);

		await page.getByLabel('Opis').fill('Najlepszy klub żeglarski');
		await page.getByRole('button', { name: 'Zapisz' }).click();

		await expect(page.getByText('Zapisano')).toBeVisible();
	});

	test('admin changes member role', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.getByRole('link', { name: 'Członkowie' }).click();
		await page.waitForURL(`**/orgs/${orgSlug}/members`);

		const memberRow = page.locator('tr', { hasText: memberName });
		await memberRow.locator('select').selectOption('captain');

		// wait for API call to complete and list to reload
		await expect(memberRow.locator('select')).toHaveValue('captain', { timeout: 10_000 });

		await page.reload();
		const updatedRow = page.locator('tr', { hasText: memberName });
		await expect(updatedRow.locator('select')).toHaveValue('captain', { timeout: 10_000 });
	});

	test('admin deletes invite', async ({ page }) => {
		await loginViaUI(page, adminEmail, adminPassword);
		await expectOrgActive(page);

		await page.getByRole('link', { name: 'Członkowie' }).click();
		await page.waitForURL(`**/orgs/${orgSlug}/members`);

		const deleteBtn = page.locator('.space-y-3').getByRole('button', { name: 'Usuń' }).first();
		if (await deleteBtn.isVisible()) {
			await deleteBtn.click();
			// wait for invite to disappear
			await expect(page.getByText('Aktywne zaproszenia')).not.toBeVisible({ timeout: 5_000 });
		}
	});
});
