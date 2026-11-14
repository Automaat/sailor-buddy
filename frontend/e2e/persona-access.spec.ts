import { test, expect, type Page } from '@playwright/test';
import {
	createTestUser,
	bootstrapTestAdmin,
	verifyTestUser,
	registerViaUI,
	loginViaUI,
	submitLoginForm,
	completeOnboardingViaUI,
	signInTestUser,
	apiRequest,
	completeOnboarding,
	SUITE_ADMIN,
	TEST_PASSWORD
} from './helpers';

// Runs against a freshly migrated local database without seed data.
// Suite admin registration stays in the UI; privilege is granted privately.

const RUN_ID = Date.now().toString(36);
const PASSWORD = TEST_PASSWORD;
const adminEmail = SUITE_ADMIN.email;
const adminName = SUITE_ADMIN.name;
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

async function signInAdmin(page: Page): Promise<void> {
	const existing = await signInTestUser(adminEmail, PASSWORD).catch(() => null);
	if (!existing) {
		await registerViaUI(page, adminName, adminEmail, PASSWORD);
		const account = await signInTestUser(adminEmail, PASSWORD);
		bootstrapTestAdmin(account.localId);
		await verifyTestUser(account.localId);
		await page.goto('/pending');
		await page.getByRole('button', { name: 'Sprawdź dostęp' }).click();
		await page.waitForURL('/');
		return;
	}
	bootstrapTestAdmin(existing.localId);
	await verifyTestUser(existing.localId);
	const me = (await apiRequest(existing.idToken, 'GET', '/auth/me')) as { onboarded: boolean };
	if (me.onboarded) {
		await loginViaUI(page, adminEmail, PASSWORD);
		return;
	}
	await submitLoginForm(page, adminEmail, PASSWORD);
	await completeOnboardingViaUI(page, 'Persona', 'Admin');
}

test.describe.serial('Single-club persona matrix', () => {
	let memberToken = '';

	test('setup: private bootstrap grants admin; signup stays member', async ({ page }) => {
		await signInAdmin(page);
		await expect(page.getByRole('link', { name: 'Członkowie' })).toHaveCount(1);
		await expect(page.getByRole('link', { name: 'Osoby', exact: true })).toHaveAttribute(
			'href',
			'/crew'
		);
		await expect(page.getByRole('link', { name: 'Moje kursy', exact: true })).toHaveAttribute(
			'href',
			'/trainings'
		);
		await expect(page.getByRole('link', { name: 'Szkolenia' })).toHaveCount(0);

		const member = await createTestUser(memberEmail, PASSWORD, 'Persona Member');
		memberToken = member.idToken;
		const memberMe = (await apiRequest(member.idToken, 'GET', '/auth/me')) as {
			id: number;
			role: string;
			membership: string;
			onboarded: boolean;
		};
		expect(memberMe).toMatchObject({ role: 'member', membership: 'gosc', onboarded: false });
		await completeOnboarding(member.idToken, 'Persona', 'Member');
		expect(await mutationStatus(member.idToken, 'GET', '/crew')).toBe(403);
		await verifyTestUser(member.localId);
		const admin = await signInTestUser(adminEmail, PASSWORD);
		await apiRequest(admin.idToken, 'PUT', `/members/${memberMe.id}/access`, {
			approved: true,
			suspended: false
		});
		memberToken = (await signInTestUser(memberEmail, PASSWORD)).idToken;
	});

	test('member: no members nav, cruises read-only, mutations 403', async ({ page }) => {
		await loginViaUI(page, memberEmail, PASSWORD);

		await expect(page.getByRole('link', { name: 'Członkowie' })).toHaveCount(0);
		await expect(page.getByRole('link', { name: 'Osoby', exact: true })).toHaveCount(0);
		await expect(page.getByRole('link', { name: 'Moje kursy', exact: true })).toHaveCount(1);

		await page.goto('/settings');
		await page.waitForURL('/profile', { timeout: 10_000 });

		await page.goto('/cruises');
		await expect(page.getByRole('link', { name: '+ Nowe wydarzenie' })).toHaveCount(0);

		// the members page redirects a non-admin away
		await page.goto('/members');
		await page.waitForURL('/', { timeout: 10_000 });

		expect(await mutationStatus(memberToken, 'POST', '/cruises', { name: 'X' })).toBe(403);
		expect(await mutationStatus(memberToken, 'POST', '/trips', { name: 'X', cruise_id: 1 })).toBe(
			403
		);
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

		await page.getByRole('link', { name: 'Rejsy', exact: true }).click();
		await page.waitForURL('/cruises');
		await page.getByRole('link', { name: '+ Nowe wydarzenie' }).click();
		await page.waitForURL('/cruises/new');

		await page.getByLabel('Nazwa wydarzenia *').fill(cruiseName);
		await page.getByRole('button', { name: 'Utwórz wydarzenie' }).click();

		await page.waitForURL(/\/cruises\/\d+/, { timeout: 10_000 });
		cruiseId = Number(page.url().split('/').pop());
		await expect(page.getByRole('heading', { name: cruiseName })).toBeVisible();
	});

	// SvelteKit reuses the page component when a link stays on the same route,
	// so the cruise page must take its data from the load function to show the
	// cruise the link points at, not the one it was opened on.
	test('admin: a link from one cruise to another shows the second', async ({ page }) => {
		const { idToken } = await signInTestUser(adminEmail, PASSWORD);
		// The name must not contain cruiseName: Playwright matches names as
		// case-insensitive substrings, here and in the later cruise-list steps.
		const otherName = `Second ${RUN_ID}`;
		const other = (await apiRequest(idToken, 'POST', '/cruises', { name: otherName })) as {
			id: number;
		};
		await loginViaUI(page, adminEmail, PASSWORD);

		await page.goto(`/cruises/${cruiseId}`);
		await expect(page.getByRole('heading', { name: cruiseName })).toBeVisible();
		await page.evaluate((href) => {
			const a = document.createElement('a');
			a.href = href;
			a.textContent = 'next cruise';
			document.querySelector('main')?.prepend(a);
		}, `/cruises/${other.id}`);
		await page.getByRole('link', { name: 'next cruise' }).click();

		await page.waitForURL(`/cruises/${other.id}`);
		await expect(page.getByRole('heading', { name: otherName })).toBeVisible();
		await expect(page.getByRole('heading', { name: cruiseName, exact: true })).toHaveCount(0);
	});

	test('admin: creates a planned trip', async ({ page }) => {
		await loginViaUI(page, adminEmail, PASSWORD);

		await page.goto(`/trips/new?cruise_id=${cruiseId}`);
		await expect(page.getByLabel('Wydarzenie klubu *')).toHaveValue(String(cruiseId));
		await page.getByLabel('Nazwa rejsu *').fill(tripName);
		await page.getByLabel('Port wyjścia').fill('Gdańsk');
		await page.getByLabel('Port docelowy').fill('Visby');
		await page.getByRole('button', { name: 'Zaplanuj rejs' }).click();

		await page.waitForURL(/\/trips\/\d+/, { timeout: 10_000 });
		tripId = Number(page.url().split('/').pop());
		await expect(page.getByText('Planowany')).toBeVisible({ timeout: 10_000 });
		await expect(page.getByRole('heading', { name: tripName })).toBeVisible();
	});

	test("admin: a link from one trip's edit page to another shows the second", async ({ page }) => {
		const { idToken } = await signInTestUser(adminEmail, PASSWORD);
		// A separate cruise keeps cruiseName fully sailed for the completed-tab step, and neither name may contain cruiseName or tripName because Playwright matches substrings. EXC:FILE011:documents-a-non-obvious-invariant
		const otherCruise = (await apiRequest(idToken, 'POST', '/cruises', {
			name: `Third ${RUN_ID}`
		})) as {
			id: number;
		};
		const otherName = `Leg ${RUN_ID}`;
		const other = (await apiRequest(idToken, 'POST', '/trips', {
			cruise_id: otherCruise.id,
			name: otherName
		})) as { id: number };
		await loginViaUI(page, adminEmail, PASSWORD);

		await page.goto(`/trips/${tripId}/edit`);
		await expect(page.getByLabel('Nazwa rejsu *')).toHaveValue(tripName);
		await page.evaluate((href) => {
			const a = document.createElement('a');
			a.href = href;
			a.textContent = 'next trip';
			document.querySelector('main')?.prepend(a);
		}, `/trips/${other.id}/edit`);
		await page.getByRole('link', { name: 'next trip' }).click();

		await page.waitForURL(`/trips/${other.id}/edit`);
		await expect(page.getByLabel('Nazwa rejsu *')).toHaveValue(otherName);
		await expect(page.getByLabel('Wydarzenie klubu *')).toHaveValue(String(otherCruise.id));
		await expect(page.getByRole('link', { name: 'Anuluj' })).toHaveAttribute(
			'href',
			`/trips/${other.id}`
		);
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

	test('fully sailed cruise appears in the completed tab', async ({ page }) => {
		await loginViaUI(page, adminEmail, PASSWORD);
		await page.getByRole('link', { name: 'Rejsy', exact: true }).click();
		await page.waitForURL('/cruises');
		await page.getByRole('tab', { name: 'Zrealizowane' }).click();
		await expect(page.getByText(cruiseName)).toBeVisible();

		await page.getByText(cruiseName).click();
		await page.waitForURL(`/cruises/${cruiseId}`);
		await expect(page.getByText(tripName)).toBeVisible();
	});

	test('member: sees the club data read-only', async ({ page }) => {
		await loginViaUI(page, memberEmail, PASSWORD);

		await page.goto('/cruises?tab=all');
		await expect(page.getByText(cruiseName)).toBeVisible();
		await expect(page.getByRole('link', { name: '+ Nowe wydarzenie' })).toHaveCount(0);

		await page.goto(`/cruises/${cruiseId}`);
		await expect(page.getByText(tripName)).toBeVisible();
		await expect(page.getByRole('link', { name: '+ Wpisz zrealizowany' })).toHaveCount(0);
	});
});

test('pending account needs verification and admin approval', async ({ page }) => {
	const email = `pending-${Date.now()}@test.local`;
	const pending = await createTestUser(email, PASSWORD, 'Pending Member');
	await completeOnboarding(pending.idToken, 'Pending', 'Member');
	await submitLoginForm(page, email, PASSWORD);
	await page.waitForURL('/pending');
	await expect(
		page.getByText('Twoje konto czeka na zatwierdzenie przez administratora.')
	).toBeVisible();
	await expect(page.getByRole('button', { name: 'Wyślij link weryfikacyjny' })).toBeVisible();
	await verifyTestUser(pending.localId);
	await page.getByRole('button', { name: 'Sprawdź dostęp' }).click();
	await expect(page.getByRole('button', { name: 'Wyślij link weryfikacyjny' })).toHaveCount(0);
	const me = (await apiRequest(pending.idToken, 'GET', '/auth/me')) as { id: number };
	const admin = await signInTestUser(adminEmail, PASSWORD);
	await apiRequest(admin.idToken, 'PUT', `/members/${me.id}/access`, {
		approved: true,
		suspended: false
	});
	await page.getByRole('button', { name: 'Sprawdź dostęp' }).click();
	await page.waitForURL('/');
	const retained = (await signInTestUser(email, PASSWORD)).idToken;
	await apiRequest(admin.idToken, 'PUT', `/members/${me.id}/access`, {
		approved: true,
		suspended: true
	});
	expect(await mutationStatus(retained, 'GET', '/cruises')).toBe(403);
});
