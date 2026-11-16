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
const startPortName = `Portowo ${RUN_ID}`;
let tripId = 0;
let cruiseId = 0;
let seriesId = 0;

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

	test.afterEach(async ({ page }, testInfo) => {
		if (testInfo.status !== testInfo.expectedStatus) {
			console.info('Persona failure:', {
				test: testInfo.title,
				url: page.url(),
				headings: await page.getByRole('heading').allTextContents(),
				navigation: await page.locator('nav a').allTextContents()
			});
		}
	});

	test('setup: private bootstrap grants admin; signup stays member', async ({ page }) => {
		await signInAdmin(page);
		await expect(page.getByRole('link', { name: 'Członkowie' })).toHaveCount(0);
		await expect(page.getByRole('link', { name: 'Jachty', exact: true })).toHaveAttribute(
			'href',
			'/yachts'
		);
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
		// A verified email is enough: no admin action precedes club access.
		await verifyTestUser(member.localId);
		memberToken = (await signInTestUser(memberEmail, PASSWORD)).idToken;
		expect(await mutationStatus(memberToken, 'GET', '/cruises')).toBe(200);
	});

	test('member: no people or yacht nav, cruises read-only, mutations 403', async ({ page }) => {
		await loginViaUI(page, memberEmail, PASSWORD);

		await expect(page.getByRole('link', { name: 'Członkowie' })).toHaveCount(0);
		await expect(page.getByRole('link', { name: 'Osoby', exact: true })).toHaveCount(0);
		await expect(page.getByRole('link', { name: 'Jachty', exact: true })).toHaveCount(0);
		await expect(page.getByRole('link', { name: 'Moje kursy', exact: true })).toHaveCount(1);

		await page.goto('/settings');
		await page.waitForURL('/profile', { timeout: 10_000 });

		await page.goto('/cruises');
		await page.waitForURL('/rejsy', { timeout: 10_000 });
		await expect(page.getByRole('heading', { name: 'Rejsy', level: 1 })).toBeVisible();
		await expect(page.getByRole('link', { name: '+ Nowy rejs' })).toHaveCount(0);
		await expect(page.getByRole('button', { name: '+ Dodaj serię' })).toHaveCount(0);

		// Legacy bookmarks retain the admin-only directory gate.
		await page.goto('/members');
		await page.waitForURL('/', { timeout: 10_000 });
		await page.goto('/crew');
		await page.waitForURL('/', { timeout: 10_000 });
		// The yacht list is for admins and the captain group only.
		await page.goto('/yachts');
		await page.waitForURL('/', { timeout: 10_000 });
		expect(await mutationStatus(memberToken, 'GET', '/yachts')).toBe(403);

		expect(await mutationStatus(memberToken, 'POST', '/cruises', { name: 'X' })).toBe(403);
		expect(await mutationStatus(memberToken, 'POST', '/trips', { name: 'X', cruise_id: 1 })).toBe(
			403
		);
		expect(await mutationStatus(memberToken, 'POST', '/yachts', { name: 'X' })).toBe(403);
		expect(await mutationStatus(memberToken, 'POST', '/crew', { full_name: 'X' })).toBe(403);
	});

	test('admin: old members URL opens people with account controls', async ({ page }) => {
		await loginViaUI(page, adminEmail, PASSWORD);
		await page.goto('/members');
		await page.waitForURL('/crew');
		await expect(page.getByRole('heading', { name: 'Osoby' })).toBeVisible();
		const memberRow = page.getByRole('row').filter({ hasText: memberEmail });
		await expect(memberRow.getByRole('link', { name: 'Persona Member' })).toBeVisible();
		await expect(memberRow.getByRole('combobox', { name: 'Dostęp: Persona Member' })).toHaveValue(
			'active'
		);
		await memberRow
			.getByRole('combobox', { name: 'Członkostwo: Persona Member' })
			.selectOption('sympatyk');
		await expect(page.getByRole('status')).toContainText('Zapisano członkostwo: Persona Member');
		await page.reload();
		await expect(
			memberRow.getByRole('combobox', { name: 'Członkostwo: Persona Member' })
		).toHaveValue('sympatyk');
	});

	test('admin: creates an edition from its series page', async ({ page }) => {
		const { idToken } = await signInTestUser(adminEmail, PASSWORD);
		const series = (await apiRequest(idToken, 'POST', '/series', {
			name: `Series ${RUN_ID}`
		})) as { id: number };
		seriesId = series.id;
		await loginViaUI(page, adminEmail, PASSWORD);

		await page.goto(`/rejsy/${seriesId}`);
		await page.getByRole('link', { name: '+ Nowa edycja' }).click();
		await page.waitForURL(`/cruises/new?series_id=${seriesId}`);
		await expect(page.getByRole('heading', { name: 'Nowa edycja' })).toBeVisible();

		await page.getByLabel('Nazwa wydarzenia *').fill(cruiseName);
		await page.getByLabel('Edycja *').fill('2026');
		await page.getByRole('button', { name: 'Utwórz edycję' }).click();

		await page.waitForURL(`/rejsy/${seriesId}`, { timeout: 10_000 });
		const row = page.getByTestId('edition-row').filter({ hasText: cruiseName });
		await expect(row).toContainText('2026');
		await row.getByRole('link', { name: cruiseName }).click();
		await page.waitForURL(/\/cruises\/\d+/, { timeout: 10_000 });
		cruiseId = Number(page.url().split('/').pop());
		await expect(page.getByRole('heading', { name: cruiseName })).toBeVisible();
	});

	test('admin: plans a standalone trip from Rejsy', async ({ page }) => {
		// The name must not contain cruiseName or tripName: Playwright matches substrings.
		const soloName = `Solo ${RUN_ID}`;
		await loginViaUI(page, adminEmail, PASSWORD);

		await page.getByRole('link', { name: 'Rejsy', exact: true }).click();
		await page.waitForURL('/rejsy');
		await expect(page.getByRole('link', { name: 'Serie', exact: true })).toHaveCount(0);
		await expect(page.locator('a[href="/cruises/new"]')).toHaveCount(0);
		await page.getByRole('link', { name: '+ Nowy rejs' }).click();
		await page.waitForURL('/trips/new');
		await expect(page.getByLabel('Seria')).toHaveValue('standalone');
		await page.getByLabel('Nazwa rejsu *').fill(soloName);
		await page.getByRole('button', { name: 'Zaplanuj rejs' }).click();

		await page.waitForURL(/\/trips\/\d+/, { timeout: 10_000 });
		const soloId = Number(page.url().split('/').pop());
		await page.goto('/cruises?tab=all');
		await page.waitForURL('/rejsy');
		const noSeries = page.getByRole('region', { name: 'Bez serii' });
		const solo = noSeries.getByRole('link', { name: new RegExp(soloName) });
		await expect(solo).toHaveAttribute('href', `/trips/${soloId}`);
		await expect(solo.getByTestId('hidden-badge')).toBeVisible();
	});

	// SvelteKit reuses the page component when a link stays on the same route,
	// so the cruise page must take its data from the load function to show the
	// cruise the link points at, not the one it was opened on.
	test('admin: a link from one cruise to another shows the second', async ({ page }) => {
		const { idToken } = await signInTestUser(adminEmail, PASSWORD);
		// The name must not contain cruiseName: Playwright matches names as
		// case-insensitive substrings, here and in the later cruise-list steps.
		const otherName = `Second ${RUN_ID}`;
		const other = (await apiRequest(idToken, 'POST', '/cruises', {
			name: otherName,
			series_id: seriesId,
			edition: 'B'
		})) as {
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

	test('admin: creates a planned trip from a suggested start port', async ({ page }) => {
		const { idToken } = await signInTestUser(adminEmail, PASSWORD);
		// A port the club visited before: the visited port of a completed standalone leg, whose name must not contain cruiseName or tripName. EXC:FILE011:documents-a-non-obvious-invariant
		const seedLeg = (await apiRequest(idToken, 'POST', '/trips', {
			name: `Seed ${RUN_ID}`
		})) as { id: number };
		const seedVoyage = (await apiRequest(idToken, 'POST', `/trips/${seedLeg.id}/complete`, {})) as {
			id: number;
		};
		await apiRequest(idToken, 'POST', `/voyages/${seedVoyage.id}/ports`, {
			name: startPortName,
			latitude: 54.35,
			longitude: 18.66
		});
		await loginViaUI(page, adminEmail, PASSWORD);

		await page.goto(`/trips/new?cruise_id=${cruiseId}`);
		await expect(page.getByLabel('Edycja *')).toHaveValue(String(cruiseId));
		await expect(page.getByLabel('Edycja *')).toBeDisabled();
		await page.getByLabel('Nazwa rejsu *').fill(tripName);
		await page.getByLabel('Port wyjścia').fill(startPortName.slice(0, 9));
		await page.getByRole('button', { name: new RegExp(`^${startPortName}`) }).click();
		await expect(page.getByLabel('Port wyjścia')).toHaveValue(startPortName);
		await expect(page.getByText('54.35000, 18.66000')).toBeVisible();
		await page.getByLabel('Port docelowy').fill('Visby');
		await page.getByLabel('Port docelowy').press('Escape');
		await page.getByRole('button', { name: 'Zaplanuj rejs' }).click();

		await page.waitForURL(/\/trips\/\d+/, { timeout: 10_000 });
		tripId = Number(page.url().split('/').pop());
		await expect(page.getByText('Nadchodzący')).toBeVisible({ timeout: 10_000 });
		await expect(page.getByRole('heading', { name: tripName })).toBeVisible();
		await expect(page.getByText(`${startPortName} → Visby`)).toBeVisible();
		const trip = (await apiRequest(idToken, 'GET', `/trips/${tripId}`)) as {
			start_port?: unknown;
			end_port?: unknown;
		};
		expect(trip.start_port).toEqual({ name: startPortName, latitude: 54.35, longitude: 18.66 });
		expect(trip.end_port).toEqual({ name: 'Visby' });
	});

	test("admin: a link from one trip's edit page to another shows the second", async ({ page }) => {
		const { idToken } = await signInTestUser(adminEmail, PASSWORD);
		// A separate cruise keeps cruiseName fully sailed for the completed-tab step, and neither name may contain cruiseName or tripName because Playwright matches substrings. EXC:FILE011:documents-a-non-obvious-invariant
		const otherCruise = (await apiRequest(idToken, 'POST', '/cruises', {
			name: `Third ${RUN_ID}`,
			series_id: seriesId,
			edition: 'C'
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
		await expect(page.getByText(`${startPortName} → Visby`)).toBeVisible();
		const { idToken } = await signInTestUser(adminEmail, PASSWORD);
		const voyage = (await apiRequest(
			idToken,
			'GET',
			`/voyages/${page.url().split('/').pop()}`
		)) as { start_port?: unknown };
		expect(voyage.start_port).toEqual({ name: startPortName, latitude: 54.35, longitude: 18.66 });
	});

	test('fully sailed cruise opens from its series on Rejsy', async ({ page }) => {
		await loginViaUI(page, adminEmail, PASSWORD);
		await page.getByRole('link', { name: 'Rejsy', exact: true }).click();
		await page.waitForURL('/rejsy');
		await page.getByRole('link', { name: `Series ${RUN_ID}` }).click();
		await page.waitForURL(`/rejsy/${seriesId}`);

		await page.getByRole('link', { name: cruiseName }).click();
		await page.waitForURL(`/cruises/${cruiseId}`);
		await expect(page.getByText(tripName)).toBeVisible();
	});

	test('member: sees the club data read-only', async ({ page }) => {
		// New editions and legs start hidden from members until an admin publishes them.
		const { idToken } = await signInTestUser(adminEmail, PASSWORD);
		await apiRequest(idToken, 'POST', `/cruises/${cruiseId}/publish`, { include_legs: true });
		await loginViaUI(page, memberEmail, PASSWORD);

		await page.goto('/rejsy');
		await expect(page.getByRole('link', { name: '+ Nowy rejs' })).toHaveCount(0);
		await page.getByRole('link', { name: `Series ${RUN_ID}` }).click();
		await page.waitForURL(`/rejsy/${seriesId}`);
		await expect(page.getByText(cruiseName)).toBeVisible();

		await page.goto(`/cruises/${cruiseId}`);
		await expect(page.getByText(tripName)).toBeVisible();
		await expect(page.getByRole('link', { name: '+ Wpisz zrealizowany' })).toHaveCount(0);
	});
});

test('new account needs only a verified email; suspension denies its token', async ({ page }) => {
	const email = `unverified-${Date.now()}@test.local`;
	const account = await createTestUser(email, PASSWORD, 'Unverified Member');
	await completeOnboarding(account.idToken, 'Unverified', 'Member');
	await submitLoginForm(page, email, PASSWORD);
	await page.waitForURL('/pending');
	await expect(page.getByRole('heading', { name: 'Potwierdź adres e-mail' })).toBeVisible();
	await expect(page.getByText(/zatwierdz/i)).toHaveCount(0);
	await expect(page.getByRole('button', { name: 'Wyślij link weryfikacyjny' })).toBeVisible();
	await verifyTestUser(account.localId);
	await page.getByRole('button', { name: 'Sprawdź dostęp' }).click();
	await page.waitForURL('/');

	const me = (await apiRequest(account.idToken, 'GET', '/auth/me')) as { id: number };
	const retained = (await signInTestUser(email, PASSWORD)).idToken;
	expect(await mutationStatus(retained, 'GET', '/cruises')).toBe(200);
	const admin = await signInTestUser(adminEmail, PASSWORD);
	await apiRequest(admin.idToken, 'PUT', `/members/${me.id}/access`, { suspended: true });
	expect(await mutationStatus(retained, 'GET', '/cruises')).toBe(403);
	await apiRequest(admin.idToken, 'PUT', `/members/${me.id}/access`, { suspended: false });
	expect(await mutationStatus(retained, 'GET', '/cruises')).toBe(200);
});
