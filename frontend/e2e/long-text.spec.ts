import { test, expect, type Page } from '@playwright/test';
import {
	apiRequest,
	bootstrapTestAdmin,
	completeOnboarding,
	createTestUser,
	loginViaUI,
	signInTestUser,
	uniqueEmail,
	verifyTestUser,
	SUITE_ADMIN,
	TEST_PASSWORD
} from './helpers';

const VIEWPORTS = [
	{ width: 390, height: 844 },
	{ width: 1440, height: 900 }
];

// 120 characters without a break opportunity.
const long = (tag: string) => tag + 'W'.repeat(120 - tag.length);
// Ports are capped at 100 characters, so they get the longest valid name.
const longPort = (tag: string) => tag + 'W'.repeat(100 - tag.length);

let paths: { admin: string[]; member: string[] };
let memberEmail: string;

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

async function mainOverflow(page: Page, path: string): Promise<number> {
	await page.goto(path);
	await page.waitForLoadState('networkidle');
	await expect(page.getByText('Wczytywanie...')).toHaveCount(0);
	return page.evaluate(() => {
		const main = document.querySelector('main');
		return main ? main.scrollWidth - main.clientWidth : 0;
	});
}

test.describe.serial('Long unbroken text', () => {
	test.beforeAll(async () => {
		const token = await adminToken();
		const captain = await post<{ id: number }>(token, '/crew', { full_name: long('Captain') });
		const person = await post<{ id: number }>(token, '/crew', {
			full_name: long('Name'),
			other_names: [long('Other')],
			email: 'e'.repeat(108) + '@example.com',
			patent_number: long('Patent')
		});
		const series = await post<{ id: number }>(token, '/series', {
			name: 'Seria z długimi wpisami',
			kind: 'sea'
		});
		const cruise = await post<{ id: number }>(token, '/cruises', {
			name: 'Edycja z długimi wpisami',
			series_id: series.id,
			edition: '2027'
		});
		const trip = await post<{ id: number }>(token, '/trips', {
			cruise_id: cruise.id,
			name: long('Trip'),
			captain_person_id: captain.id,
			description: long('Desc'),
			countries: long('Countries'),
			start_port: longPort('Start'),
			end_port: longPort('End'),
			embark_date: '2027-07-01',
			disembark_date: '2027-07-08',
			max_crew: 8
		});
		const voyage = await post<{ id: number }>(token, '/voyages', {
			cruise_id: cruise.id,
			name: long('Voyage'),
			captain_person_id: captain.id,
			description: long('VDesc'),
			countries: long('VCountries'),
			start_port: longPort('VStart'),
			end_port: longPort('VEnd'),
			embark_date: '2025-06-01',
			disembark_date: '2025-06-08',
			year: 2025
		});
		await apiRequest(token, 'PUT', `/voyages/${voyage.id}/log`, {
			captain_notes: long('Notes'),
			voyage_card: long('Card')
		});
		await post(token, `/voyages/${voyage.id}/crew`, {
			crew_member_id: person.id,
			role: 'zalogant'
		});
		await post(token, `/cruises/${cruise.id}/publish`, { include_legs: true });
		await post(token, `/trips/${trip.id}/enroll-token`);
		const signUp = await post<{ id: number }>(token, `/trips/${trip.id}/enrollments`, {
			crew_member_id: person.id,
			note: long('AdminNote')
		});
		await apiRequest(token, 'PUT', `/trips/${trip.id}/enrollments/${signUp.id}/status`, {
			status: 'accepted'
		});

		memberEmail = uniqueEmail('long-text-member');
		const member = await createTestUser(memberEmail, TEST_PASSWORD, 'Członek');
		await verifyTestUser(member.localId);
		const verified = await signInTestUser(memberEmail, TEST_PASSWORD);
		await completeOnboarding(verified.idToken, 'Członek', 'Długi');
		await post(verified.idToken, `/trips/${trip.id}/sign-up`);
		await apiRequest(verified.idToken, 'PUT', `/trips/${trip.id}/sign-up/note`, {
			note: long('OwnNote')
		});

		paths = {
			admin: [
				`/cruises/${cruise.id}`,
				`/trips/${trip.id}`,
				`/trips/${trip.id}/enrollments`,
				`/voyages/${voyage.id}`,
				`/crew/${person.id}`
			],
			member: [`/cruises/${cruise.id}`, `/trips/${trip.id}`, `/voyages/${voyage.id}`]
		};
	});

	for (const role of ['admin', 'member'] as const) {
		test(`${role} detail pages wrap instead of widening the panel`, async ({ page }) => {
			await loginViaUI(page, role === 'admin' ? SUITE_ADMIN.email : memberEmail, TEST_PASSWORD);
			for (const viewport of VIEWPORTS) {
				await page.setViewportSize(viewport);
				for (const path of paths[role]) {
					expect.soft(await mainOverflow(page, path), `${path} at ${viewport.width}`).toBe(0);
				}
			}
		});
	}
});
