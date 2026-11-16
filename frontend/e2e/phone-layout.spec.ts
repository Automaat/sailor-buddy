import { test, expect, type Page } from '@playwright/test';
import {
	apiRequest,
	bootstrapTestAdmin,
	verifyTestUser,
	completeOnboarding,
	createTestUser,
	loginViaUI,
	signInTestUser,
	uniqueEmail,
	SUITE_ADMIN,
	TEST_PASSWORD
} from './helpers';

const PHONE = { width: 390, height: 844 };
const TABLET = { width: 768, height: 1024 };
const DESKTOP = { width: 1440, height: 900 };
const MIN_TAP = 44;

type Ids = {
	cruise: number;
	trip: number;
	voyage: number;
	series: number;
	crew: number;
	token: string;
};

let ids: Ids;

async function adminToken(): Promise<string> {
	const account =
		(await signInTestUser(SUITE_ADMIN.email, TEST_PASSWORD).catch(() => null)) ??
		(await createTestUser(SUITE_ADMIN.email, TEST_PASSWORD, SUITE_ADMIN.name));
	await apiRequest(account.idToken, 'GET', '/auth/me');
	bootstrapTestAdmin(account.localId);
	await verifyTestUser(account.localId);
	const verified = await signInTestUser(SUITE_ADMIN.email, TEST_PASSWORD);
	await completeOnboarding(verified.idToken, 'Persona', 'Admin');
	return verified.idToken;
}

async function post<T>(token: string, path: string, body?: unknown): Promise<T> {
	return (await apiRequest(token, 'POST', path, body)) as T;
}

// A planned leg's crew comes only from accepted sign-ups: sign the person
// up, accept the sign-up, then set the crew entry's role.
async function addLegCrew(token: string, tripId: number, personId: number, role: string) {
	const signUp = await post<{ id: number }>(token, `/trips/${tripId}/enrollments`, {
		crew_member_id: personId
	});
	await apiRequest(token, 'PUT', `/trips/${tripId}/enrollments/${signUp.id}/status`, {
		status: 'accepted'
	});
	const crew = (await apiRequest(token, 'GET', `/trips/${tripId}/crew`)) as {
		id: number;
		crew_member_id: number;
	}[];
	const entry = crew.find((c) => c.crew_member_id === personId);
	if (!entry) throw new Error(`person ${personId} is not on the crew of leg ${tripId}`);
	await apiRequest(token, 'PATCH', `/trips/${tripId}/crew/${entry.id}`, { role });
}

// main scrolls on its own, so content wider than the screen widens main's
// scroll area without widening the document.
async function pageWidth(page: Page, path: string): Promise<number> {
	await page.goto(path);
	await page.waitForLoadState('networkidle');
	await expect(page.getByText('Wczytywanie...')).toHaveCount(0);
	return page.evaluate(() => {
		const main = document.querySelector('main');
		const mainOverflow = main ? main.scrollWidth - main.clientWidth : 0;
		return document.documentElement.scrollWidth + Math.max(0, mainOverflow);
	});
}

async function smallTapTargets(page: Page): Promise<string[]> {
	return page.evaluate((min) => {
		const els = document.querySelectorAll<HTMLElement>(
			'button, select, a.tap-target, header a, #mobile-menu a, main a.inline-block, main a[class*="px-"], .leaflet-bar a'
		);
		const small: string[] = [];
		for (const el of els) {
			const r = el.getBoundingClientRect();
			if (r.width === 0 || r.height === 0) continue;
			if (r.height < min || r.width < min) {
				small.push(`${el.tagName} "${el.textContent?.trim()}" ${r.width}x${r.height}`);
			}
		}
		return small;
	}, MIN_TAP);
}

// squeezedFields lists form controls narrower than half the form or not
// below their label: a stray grid column squeezes fields into slivers and
// runs their labels into each other.
async function squeezedFields(page: Page): Promise<string[]> {
	return page.evaluate(() => {
		const form = document.querySelector<HTMLFormElement>('main form');
		if (!form) return ['no form'];
		const half = form.clientWidth / 2;
		const squeezed: string[] = [];
		for (const label of form.querySelectorAll<HTMLLabelElement>('label[for]')) {
			const control = document.getElementById(label.htmlFor);
			if (!control || ['checkbox', 'radio'].includes(control.getAttribute('type') ?? '')) continue;
			const c = control.getBoundingClientRect();
			if (c.width === 0) continue;
			const l = label.getBoundingClientRect();
			if (c.width < half || c.top < l.bottom - 1) {
				squeezed.push(`${label.textContent?.trim()} ${c.width}x${c.height} at ${c.top}`);
			}
		}
		return squeezed;
	});
}

test.describe.serial('Phone layout', () => {
	test.beforeAll(async () => {
		const token = await adminToken();
		const series = await post<{ id: number }>(token, '/series', {
			name: 'Seria mobilna',
			kind: 'sea'
		});
		// An edition holds only edition-wide fields; dates, ports and prices live on its legs.
		const cruise = await post<{ id: number }>(token, '/cruises', {
			name: 'Wydarzenie z bardzo długą nazwą na telefon',
			series_id: series.id,
			edition: '2027',
			countries: 'Polska, Dania, Szwecja',
			description: 'Opis wydarzenia, który jest na tyle długi, że musi się zawinąć.',
			included: ['Jacht z pełnym wyposażeniem i ubezpieczeniem'],
			not_included: ['Dojazd do portu zaokrętowania']
		});
		const trip = await post<{ id: number }>(token, '/trips', {
			cruise_id: cruise.id,
			name: 'Etap pierwszy',
			embark_date: '2027-07-01',
			disembark_date: '2027-07-07',
			start_port: { name: 'Gdańsk', latitude: 54.35, longitude: 18.66 },
			end_port: { name: 'Bornholm' },
			captain_name: 'Kapitan Bardzo-Długie-Nazwisko',
			max_crew: 8,
			cost_total: 1234500,
			deposit: 50000
		});
		const leg = await post<{ id: number }>(token, '/trips', {
			cruise_id: cruise.id,
			name: 'Etap drugi',
			embark_date: '2027-07-07',
			disembark_date: '2027-07-14',
			start_port: { name: 'Bornholm' },
			end_port: { name: 'Visby' }
		});
		// New editions and legs start hidden; the member flows below need them published.
		await post(token, `/cruises/${cruise.id}/publish`, { include_legs: true });
		const longCrew = await post<{ id: number }>(token, '/crew', {
			full_name: 'Aleksandra Konstancja Brzęczyszczykiewicz-Wolańczyk',
			email: 'konstanty.brzeczyszczykiewicz@student.agh.edu.pl'
		});
		// The long name widens the people pickers on the sign-up and voyage
		// pages; the unbreakable email widens the /crew card.
		await addLegCrew(token, leg.id, longCrew.id, 'Załoga');
		const voyage = await post<{ id: number }>(token, `/trips/${leg.id}/complete`, {
			hours_sail: 30,
			hours_engine: 18,
			miles: 120
		});
		const crew = await post<{ id: number }>(token, '/crew', {
			full_name: 'Załogant Testowy',
			email: 'zalogant-z-dlugim-adresem@test.local'
		});
		await addLegCrew(token, trip.id, crew.id, 'Sternik');
		// A standalone leg fills the Rejsy page's "Bez serii" group; its
		// unbreakable name must wrap inside the card.
		await post(token, '/trips', {
			name: 'Samodzielny-rejs-bez-serii-z-bardzo-długą-nazwą-bez-spacji',
			embark_date: '2027-08-01',
			disembark_date: '2027-08-08',
			start_port: { name: 'Świnoujście' },
			end_port: { name: 'Kopenhaga' }
		});
		await post(token, '/yachts', { name: 'Jacht', yacht_type: 'Slup', length_m: 12 });
		await post(token, '/trainings', { name: 'Szkolenie', date: '2027-03-01' });
		const { token: enrollToken } = await post<{ token: string }>(
			token,
			`/trips/${trip.id}/enroll-token`
		);

		const member = await createTestUser(uniqueEmail('phone-member'), TEST_PASSWORD, 'Członek');
		await completeOnboarding(member.idToken, 'Członek', 'Mobilny');
		await post(member.idToken, `/enroll/${enrollToken}`, {});

		ids = {
			cruise: cruise.id,
			trip: trip.id,
			voyage: voyage.id,
			series: series.id,
			crew: crew.id,
			token: enrollToken
		};
	});

	test('logged-out pages fit the screen', async ({ page }) => {
		await page.setViewportSize(PHONE);
		for (const path of ['/login', `/enroll/${ids.token}`]) {
			expect.soft(await pageWidth(page, path), path).toBeLessThanOrEqual(PHONE.width);
		}
	});

	test('onboarding fits the screen', async ({ page }) => {
		await page.setViewportSize(PHONE);
		const email = uniqueEmail('phone-onboarding');
		await createTestUser(email, TEST_PASSWORD, 'Nowy');
		await page.goto('/login');
		await page.getByLabel('Email').fill(email);
		await page.getByLabel('Hasło').fill(TEST_PASSWORD);
		await page.getByRole('button', { name: 'Zaloguj' }).click();
		await page.waitForURL(/\/onboarding/);
		expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(
			PHONE.width
		);
	});

	test('every signed-in page fits the screen', async ({ page }) => {
		await page.setViewportSize(PHONE);
		await loginViaUI(page, SUITE_ADMIN.email, TEST_PASSWORD);
		const paths = [
			'/',
			`/cruises/new?series_id=${ids.series}`,
			`/cruises/${ids.cruise}`,
			`/cruises/${ids.cruise}/edit`,
			'/trips/new',
			`/trips/${ids.trip}`,
			`/trips/${ids.trip}/edit`,
			`/trips/${ids.trip}/enrollments`,
			'/voyages/new',
			`/voyages/${ids.voyage}`,
			`/voyages/${ids.voyage}/edit`,
			'/rejsy',
			`/rejsy/${ids.series}`,
			'/yachts',
			'/crew',
			`/crew/${ids.crew}`,
			'/trainings',
			'/members',
			'/profile'
		];
		for (const path of paths) {
			expect.soft(await pageWidth(page, path), path).toBeLessThanOrEqual(PHONE.width);
			expect.soft(await smallTapTargets(page), `${path} tap targets`).toEqual([]);
		}
	});

	test('trip forms keep one full-width column', async ({ page }) => {
		await page.setViewportSize(PHONE);
		await loginViaUI(page, SUITE_ADMIN.email, TEST_PASSWORD);
		for (const path of ['/trips/new', `/trips/${ids.trip}/edit`]) {
			await page.goto(path);
			await expect(page.getByLabel('Nazwa rejsu *')).toBeVisible();
			expect.soft(await squeezedFields(page), path).toEqual([]);
		}
	});

	test('people list shows every account control without sideways scrolling', async ({ page }) => {
		await page.setViewportSize(PHONE);
		await loginViaUI(page, SUITE_ADMIN.email, TEST_PASSWORD);
		await page.goto('/crew');
		await expect(page.getByRole('combobox', { name: /^Rola:/ }).first()).toBeVisible();

		const search = await page.getByRole('searchbox', { name: 'Szukaj osób' }).boundingBox();
		const list = page.getByTestId('person').first().locator('xpath=ancestor::div[1]');
		const listBox = await list.boundingBox();
		expect(search!.width).toBeGreaterThanOrEqual(listBox!.width - 1);
		expect(await list.evaluate((el) => el.scrollWidth - el.clientWidth)).toBe(0);

		const controls = page.getByTestId('person').locator('select, input[type="checkbox"]');
		expect(await controls.count()).toBeGreaterThan(0);
		for (const box of await controls.evaluateAll((els) =>
			els.map((el) => {
				const r = el.getBoundingClientRect();
				return { label: el.getAttribute('aria-label'), left: r.left, right: r.right };
			})
		)) {
			expect.soft(box.left, `${box.label}`).toBeGreaterThanOrEqual(0);
			expect.soft(box.right, `${box.label}`).toBeLessThanOrEqual(PHONE.width);
		}
	});

	test('detail headers and sign-up rows fit beside the tablet sidebar', async ({ page }) => {
		await page.setViewportSize(TABLET);
		await loginViaUI(page, SUITE_ADMIN.email, TEST_PASSWORD);
		const paths = [
			`/trips/${ids.trip}`,
			`/trips/${ids.trip}/enrollments`,
			`/crew/${ids.crew}`,
			'/crew',
			'/trips/new'
		];
		for (const path of paths) {
			expect.soft(await pageWidth(page, path), path).toBeLessThanOrEqual(TABLET.width);
		}
		await page.goto(`/trips/${ids.trip}/enrollments`);
		for (const name of ['Załogant Testowy', 'Członek Mobilny']) {
			const lines = await page
				.getByText(name, { exact: true })
				.evaluate((el) => el.getClientRects().length);
			expect.soft(lines, `${name} line boxes`).toBe(1);
		}
	});

	test('menu replaces the sidebar and marks the current section', async ({ page }) => {
		await page.setViewportSize(PHONE);
		await loginViaUI(page, SUITE_ADMIN.email, TEST_PASSWORD);
		await page.goto(`/trips/${ids.trip}`);

		const menuButton = page.getByRole('button', { name: 'Otwórz menu' });
		await expect(menuButton).toBeVisible();
		await expect(page.getByRole('link', { name: 'Rejsy', exact: true })).toBeHidden();

		await menuButton.click();
		const menu = page.locator('#mobile-menu');
		await expect(menu.getByRole('link', { name: 'Rejsy' })).toHaveAttribute('aria-current', 'page');
		await expect(menu.getByRole('link', { name: SUITE_ADMIN.name })).toBeVisible();
		await expect(menu.getByRole('button', { name: 'Wyloguj' })).toBeVisible();
		expect(await smallTapTargets(page)).toEqual([]);

		await menu.getByRole('link', { name: 'Jachty' }).click();
		await page.waitForURL('/yachts');
		await expect(menu).toBeHidden();

		await page.getByRole('button', { name: 'Otwórz menu' }).click();
		await expect(menu).toBeVisible();
		await page.keyboard.press('Escape');
		await expect(menu).toBeHidden();
	});

	test('detail rows stack label above value', async ({ page }) => {
		await page.setViewportSize(PHONE);
		await loginViaUI(page, SUITE_ADMIN.email, TEST_PASSWORD);
		const rows: [string, string][] = [
			[`/trips/${ids.trip}`, 'Daty'],
			[`/trips/${ids.trip}`, 'Kaucja (zwrotna)'],
			[`/voyages/${ids.voyage}`, 'Daty']
		];
		for (const [path, term] of rows) {
			await page.goto(path);
			const dt = page.locator('dt', { hasText: term }).first();
			const label = await dt.boundingBox();
			const value = await dt.locator('xpath=following-sibling::dd[1]').boundingBox();
			expect(label, `${path} ${term}`).not.toBeNull();
			expect(value, `${path} ${term}`).not.toBeNull();
			expect(value!.y, `${path} ${term}`).toBeGreaterThanOrEqual(label!.y + label!.height);
		}
	});

	test('cruise enrollment links are tappable', async ({ page }) => {
		await page.setViewportSize(PHONE);
		await loginViaUI(page, SUITE_ADMIN.email, TEST_PASSWORD);
		await page.goto(`/cruises/${ids.cruise}`);
		const link = page.getByRole('link', { name: 'Zarządzaj zapisami' }).first();
		const box = await link.boundingBox();
		expect(box).not.toBeNull();
		expect(box!.height).toBeGreaterThanOrEqual(MIN_TAP);
	});

	test('desktop keeps the sidebar', async ({ page }) => {
		await page.setViewportSize(DESKTOP);
		await loginViaUI(page, SUITE_ADMIN.email, TEST_PASSWORD);
		await expect(page.getByRole('link', { name: 'Rejsy', exact: true })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Otwórz menu' })).toBeHidden();
	});
});
