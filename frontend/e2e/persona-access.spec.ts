import { test, expect, type Page } from '@playwright/test';
import { createTestUser, loginViaUI, clearFirebaseUsers, apiRequest } from './helpers';

const RUN_ID = Date.now().toString(36);
const PASSWORD = 'TestPass123!';

const orgSlug = `pa-club-${RUN_ID}`;
const orgDisplayName = `PA Club ${RUN_ID}`;

type OrgRole = 'admin' | 'captain' | 'crew';

type Persona = {
	key: string;
	// org role, or null for a user that belongs to no organization
	role: OrgRole | null;
	email: string;
	name: string;
	// nav: club-only entry (Wydarzenia)
	seesClubNav: boolean;
	// nav: admin-only entries (Członkowie, Ustawienia)
	seesAdminNav: boolean;
	// /cruises: can start a new cruise
	canManageCruises: boolean;
};

const personas: Persona[] = [
	{
		key: 'admin',
		role: 'admin',
		email: `pa-admin-${RUN_ID}@test.local`,
		name: 'Persona Admin',
		seesClubNav: true,
		seesAdminNav: true,
		canManageCruises: true
	},
	{
		key: 'captain',
		role: 'captain',
		email: `pa-captain-${RUN_ID}@test.local`,
		name: 'Persona Captain',
		seesClubNav: true,
		seesAdminNav: false,
		canManageCruises: false
	},
	{
		key: 'crew',
		role: 'crew',
		email: `pa-crew-${RUN_ID}@test.local`,
		name: 'Persona Crew',
		seesClubNav: true,
		seesAdminNav: false,
		canManageCruises: false
	},
	{
		key: 'solo',
		role: null,
		email: `pa-solo-${RUN_ID}@test.local`,
		name: 'Persona Solo',
		seesClubNav: false,
		seesAdminNav: false,
		canManageCruises: false
	}
];

// An org member has their org auto-selected on login; wait for the nav to settle.
async function settleOrgContext(page: Page, persona: Persona) {
	if (persona.role) {
		await expect(page.locator('nav').getByText(orgDisplayName)).toBeVisible();
	}
}

test.describe.serial('Persona access matrix', () => {
	test.beforeAll(async () => {
		await clearFirebaseUsers();
	});

	test('setup: provision personas, org and role assignments', async () => {
		// create every persona via the Firebase API; the backend auto-provisions
		// the DB user on the first authenticated request
		const tokens: Record<string, string> = {};
		for (const persona of personas) {
			const user = await createTestUser(persona.email, PASSWORD, persona.name);
			tokens[persona.key] = user.idToken;
		}

		// admin creates the org (creator becomes admin) and role-scoped invites
		await apiRequest(tokens.admin, 'POST', '/orgs', {
			name: orgDisplayName,
			slug: orgSlug
		});
		await apiRequest(tokens.admin, 'POST', `/orgs/${orgSlug}/invites`, { role: 'captain' });
		await apiRequest(tokens.admin, 'POST', `/orgs/${orgSlug}/invites`, { role: 'crew' });

		const invites = (await apiRequest(
			tokens.admin,
			'GET',
			`/orgs/${orgSlug}/invites`
		)) as Array<{ token: string; role: string }>;
		const capToken = invites.find((i) => i.role === 'captain')!.token;
		const crewToken = invites.find((i) => i.role === 'crew')!.token;

		// captain and crew join with their role-scoped invites
		await apiRequest(tokens.captain, 'POST', `/join/${capToken}`);
		await apiRequest(tokens.crew, 'POST', `/join/${crewToken}`);
		// solo persona intentionally joins no org
	});

	for (const persona of personas) {
		test(`${persona.key}: navigation reflects role`, async ({ page }) => {
			await loginViaUI(page, persona.email, PASSWORD);
			await settleOrgContext(page, persona);

			await expect(page.getByRole('link', { name: 'Wydarzenia' })).toHaveCount(
				persona.seesClubNav ? 1 : 0
			);
			await expect(page.getByRole('link', { name: 'Członkowie' })).toHaveCount(
				persona.seesAdminNav ? 1 : 0
			);
			await expect(page.getByRole('link', { name: 'Ustawienia' })).toHaveCount(
				persona.seesAdminNav ? 1 : 0
			);
		});

		test(`${persona.key}: cruise creation gated by role`, async ({ page }) => {
			await loginViaUI(page, persona.email, PASSWORD);
			await settleOrgContext(page, persona);

			await page.goto('/cruises');
			await expect(page.getByRole('link', { name: '+ Nowe wydarzenie' })).toHaveCount(
				persona.canManageCruises ? 1 : 0
			);
		});

		test(`${persona.key}: org admin pages gated by role`, async ({ page }) => {
			await loginViaUI(page, persona.email, PASSWORD);
			await settleOrgContext(page, persona);

			if (persona.seesAdminNav) {
				await page.goto(`/orgs/${orgSlug}/members`);
				await page.waitForURL(`**/orgs/${orgSlug}/members`);
				await expect(page.getByRole('heading', { name: 'Członkowie' })).toBeVisible();
			} else if (persona.role) {
				// non-admin org member is redirected away from admin-only pages
				await page.goto(`/orgs/${orgSlug}/members`);
				await page.waitForURL('/', { timeout: 10_000 });

				await page.goto(`/orgs/${orgSlug}/settings`);
				await page.waitForURL('/', { timeout: 10_000 });
			}
			// solo persona is not an org member — covered by the navigation test
		});
	}
});
