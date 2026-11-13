import { test, expect, type Page } from '@playwright/test';
import {
	apiRequest,
	completeOnboarding,
	createTestUser,
	loginViaUI,
	signInTestUser,
	uniqueEmail,
	SUITE_ADMIN,
	TEST_PASSWORD
} from './helpers';

const PHONE = { width: 390, height: 844 };
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
	const signedIn = await signInTestUser(SUITE_ADMIN.email, TEST_PASSWORD).catch(() => null);
	if (signedIn) return signedIn.idToken;
	// Only the first account the backend provisions becomes admin, so a run of
	// this spec alone works on a fresh database and fails loudly otherwise.
	const created = await createTestUser(SUITE_ADMIN.email, TEST_PASSWORD, SUITE_ADMIN.name);
	const me = (await apiRequest(created.idToken, 'GET', '/auth/me')) as { role: string };
	if (me.role !== 'admin') {
		throw new Error('phone-layout needs the suite admin: run it with persona-access.spec.ts');
	}
	await completeOnboarding(created.idToken, 'Persona', 'Admin');
	return created.idToken;
}

async function post<T>(token: string, path: string, body?: unknown): Promise<T> {
	return (await apiRequest(token, 'POST', path, body)) as T;
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
		const els = document.querySelectorAll<HTMLElement>('button, a.tap-target, header a, #mobile-menu a, main a.inline-block, main a[class*="px-"]');
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

test.describe.serial('Phone layout', () => {
	test.beforeAll(async () => {
		const token = await adminToken();
		const series = await post<{ id: number }>(token, '/series', { name: 'Seria mobilna' });
		const cruise = await post<{ id: number }>(token, '/cruises', {
			name: 'Wydarzenie z bardzo długą nazwą na telefon',
			series_id: series.id,
			embark_date: '2027-07-01',
			disembark_date: '2027-07-14',
			start_port: 'Gdańsk',
			end_port: 'Visby',
			description: 'Opis wydarzenia, który jest na tyle długi, że musi się zawinąć.',
			deposit: 50000
		});
		const trip = await post<{ id: number }>(token, '/trips', {
			cruise_id: cruise.id,
			name: 'Etap pierwszy',
			embark_date: '2027-07-01',
			disembark_date: '2027-07-07',
			start_port: 'Gdańsk',
			end_port: 'Bornholm',
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
			start_port: 'Bornholm',
			end_port: 'Visby'
		});
		const voyage = await post<{ id: number }>(token, `/trips/${leg.id}/complete`, {
			hours_sail: 30,
			hours_engine: 18,
			miles: 120
		});
		const crew = await post<{ id: number }>(token, '/crew', {
			full_name: 'Załogant Testowy',
			email: 'zalogant-z-dlugim-adresem@test.local'
		});
		await post(token, `/trips/${trip.id}/crew`, { crew_member_id: crew.id, role: 'Sternik' });
		await post(token, '/yachts', { name: 'Jacht', yacht_type: 'Slup', length_m: 12 });
		await post(token, '/trainings', { name: 'Szkolenie', date: '2027-03-01' });
		const { token: enrollToken } = await post<{ token: string }>(
			token,
			`/trips/${trip.id}/enroll-token`
		);

		const member = await createTestUser(uniqueEmail('phone-member'), TEST_PASSWORD, 'Członek');
		await completeOnboarding(member.idToken, 'Członek', 'Mobilny');
		await post(member.idToken, `/trips/${trip.id}/sign-up`);

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
			'/cruises',
			'/cruises?tab=all',
			'/cruises/new',
			`/cruises/${ids.cruise}`,
			`/cruises/${ids.cruise}/edit`,
			'/trips/new',
			`/trips/${ids.trip}`,
			`/trips/${ids.trip}/edit`,
			`/trips/${ids.trip}/enrollments`,
			'/voyages/new',
			`/voyages/${ids.voyage}`,
			`/voyages/${ids.voyage}/edit`,
			'/series',
			`/series/${ids.series}`,
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

	test('menu replaces the sidebar and marks the current section', async ({ page }) => {
		await page.setViewportSize(PHONE);
		await loginViaUI(page, SUITE_ADMIN.email, TEST_PASSWORD);
		await page.goto(`/trips/${ids.trip}`);

		const menuButton = page.getByRole('button', { name: 'Otwórz menu' });
		await expect(menuButton).toBeVisible();
		await expect(page.getByRole('link', { name: 'Rejsy' })).toBeHidden();

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
			[`/voyages/${ids.voyage}`, 'Daty'],
			[`/cruises/${ids.cruise}`, 'Kaucja (zwrotna)']
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

	test('desktop keeps the sidebar', async ({ page }) => {
		await page.setViewportSize(DESKTOP);
		await loginViaUI(page, SUITE_ADMIN.email, TEST_PASSWORD);
		await expect(page.getByRole('link', { name: 'Rejsy' })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Otwórz menu' })).toBeHidden();
	});
});
