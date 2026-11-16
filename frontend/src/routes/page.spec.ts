import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/svelte';
import type { DashboardStats, MyLeg, SeaTime } from '#lib/api/types.js';

vi.mock('#lib/api/routes.js', () => ({ getDashboard: vi.fn(), listMyLegs: vi.fn() }));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: false, user: { id: 2, role: 'member' } }
}));

import { getDashboard, listMyLegs } from '#lib/api/routes.js';
import { auth } from '#lib/stores/auth.svelte.js';
import DashboardPage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

const mockGet = vi.mocked(getDashboard);
const mockLegs = vi.mocked(listMyLegs);
const mockAuth = auth as unknown as { isAdmin: boolean; user: { is_captain?: boolean } };

function runLoad() {
	return load({} as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

async function renderPage() {
	return render(DashboardPage, { props: { data: await runLoad() } });
}

function leg(overrides: Partial<MyLeg> = {}): MyLeg {
	return {
		id: 7,
		name: 'Etap 1',
		cruise_id: 3,
		cruise_name: 'Sycylia 2027',
		relation: 'crew',
		...overrides
	};
}

function seaTime(overrides: Partial<SeaTime> = {}): SeaTime {
	return {
		voyage_count: 0,
		total_hours: 0,
		total_hours_sail: 0,
		total_hours_engine: 0,
		total_miles: 0,
		total_days: 0,
		...overrides
	};
}

function emptyPersonal(): DashboardStats['personal'] {
	return {
		total: seaTime(),
		as_captain: seaTime(),
		as_crew: seaTime(),
		by_year: [],
		voyages: []
	};
}

const sailor: DashboardStats = {
	personal: {
		total: seaTime({
			voyage_count: 2,
			total_miles: 320,
			total_hours: 80,
			total_hours_sail: 60,
			total_hours_engine: 20,
			total_days: 10
		}),
		as_captain: seaTime({
			voyage_count: 1,
			total_miles: 200,
			total_hours: 50,
			total_days: 6
		}),
		as_crew: seaTime({
			voyage_count: 1,
			total_miles: 120,
			total_hours: 30,
			total_days: 4
		}),
		by_year: [
			{
				year: 2024,
				voyage_count: 1,
				total_miles: 200,
				total_hours: 50,
				total_days: 6
			},
			{
				year: 2025,
				voyage_count: 1,
				total_miles: 120,
				total_hours: 30,
				total_days: 4
			}
		],
		voyages: [
			{
				id: 12,
				name: 'Bornholm',
				embark_date: '2025-06-01',
				disembark_date: '2025-06-05',
				yacht_name: 'Zjawa',
				role: 'crew',
				miles: 120,
				hours_total: 30,
				days: 4
			},
			{
				id: 11,
				name: 'Gotland',
				year: 2024,
				role: 'captain',
				miles: 200,
				hours_total: 50,
				days: 6
			}
		]
	}
};

beforeEach(() => {
	mockGet.mockReset();
	mockLegs.mockReset().mockResolvedValue([]);
	mockAuth.isAdmin = false;
	mockAuth.user.is_captain = false;
	vi.spyOn(console, 'error').mockImplementation(() => {});
});

describe('dashboard page', () => {
	it("shows the user's own totals, role split, years and voyages", async () => {
		mockGet.mockResolvedValue(sailor);
		await renderPage();

		const mine = await screen.findByRole('region', { name: 'Mój staż' });
		expect(within(mine).getByText('Zrealizowane rejsy').nextElementSibling).toHaveTextContent('2');
		expect(within(mine).getByText('Mile morskie').nextElementSibling).toHaveTextContent('320');
		expect(within(mine).getByText('60h żagle / 20h silnik')).toBeInTheDocument();
		expect(within(mine).getByText('Jako kapitan').nextElementSibling).toHaveTextContent(
			'1 rejs · 200 Mm · 50 h · 6 dni'
		);
		expect(within(mine).getByText('Jako załoga').nextElementSibling).toHaveTextContent(
			'1 rejs · 120 Mm · 30 h · 4 dni'
		);

		const years = within(mine).getAllByRole('row');
		expect(years[1]).toHaveTextContent('2024');
		expect(years[2]).toHaveTextContent('2025');

		const bornholm = within(mine).getByRole('link', { name: /Bornholm/ });
		expect(bornholm).toHaveAttribute('href', '/voyages/12');
		expect(bornholm).toHaveTextContent('1–5 cze 2025 · 5 dni');
		expect(bornholm).toHaveTextContent('Zjawa');
		expect(bornholm).toHaveTextContent('Załoga · 120 Mm · 30 h');
		const gotland = within(mine).getByRole('link', { name: /Gotland/ });
		expect(gotland).toHaveAttribute('href', '/voyages/11');
		expect(gotland).toHaveTextContent('Kapitan · 200 Mm · 50 h');
	});

	it.each([
		[0, '0 rejsów', '0 dni'],
		[1, '1 rejs', '1 dzień'],
		[2, '2 rejsy', '2 dni'],
		[5, '5 rejsów', '5 dni'],
		[12, '12 rejsów', '12 dni']
	])('uses Polish plurals in the role split for %i', async (n, voyages, days) => {
		const split = seaTime({ voyage_count: n, total_days: n });
		mockGet.mockResolvedValue({
			personal: { ...sailor.personal, as_captain: split, as_crew: split }
		});
		await renderPage();

		const mine = await screen.findByRole('region', { name: 'Mój staż' });
		for (const label of ['Jako kapitan', 'Jako załoga']) {
			expect(within(mine).getByText(label).nextElementSibling).toHaveTextContent(
				`${voyages} · 0 Mm · 0 h · ${days}`
			);
		}
	});

	it('hides the club section when the response has none (member)', async () => {
		mockGet.mockResolvedValue(sailor);
		await renderPage();
		await screen.findByRole('region', { name: 'Mój staż' });
		expect(screen.queryByRole('region', { name: 'Klub' })).not.toBeInTheDocument();
	});

	it('shows labelled club totals for an admin', async () => {
		mockAuth.isAdmin = true;
		mockGet.mockResolvedValue({
			...sailor,
			club: {
				total: seaTime({ voyage_count: 9, total_miles: 2775 }),
				by_year: [
					{
						year: 2023,
						voyage_count: 9,
						total_miles: 2775,
						total_hours: 400,
						total_days: 40
					}
				]
			}
		});
		await renderPage();

		const club = await screen.findByRole('region', { name: 'Klub' });
		expect(within(club).getByText('Zrealizowane rejsy').nextElementSibling).toHaveTextContent('9');
		expect(within(club).getByText('Mile morskie').nextElementSibling).toHaveTextContent(
			(2775).toLocaleString()
		);
		expect(within(club).getAllByRole('row')[1]).toHaveTextContent('2023');
		const mine = screen.getByRole('region', { name: 'Mój staż' });
		expect(within(mine).getByText('Zrealizowane rejsy').nextElementSibling).toHaveTextContent('2');
	});

	it('shows an empty state linking to cruises for a user with no voyages', async () => {
		mockGet.mockResolvedValue({ personal: emptyPersonal() });
		await renderPage();

		const mine = await screen.findByRole('region', { name: 'Mój staż' });
		expect(
			within(mine).getByText('Nie masz jeszcze zrealizowanych rejsów z klubem.')
		).toBeInTheDocument();
		expect(within(mine).getByRole('link', { name: 'Zobacz rejsy klubu' })).toHaveAttribute(
			'href',
			'/rejsy'
		);
		expect(within(mine).queryByText('Zrealizowane rejsy')).not.toBeInTheDocument();
	});

	it('names the failed statistics in the banner instead of showing them', async () => {
		mockGet.mockRejectedValue(new Error('Request failed: 500'));
		await renderPage();

		expect(screen.getByRole('alert')).toHaveTextContent('Nie udało się wczytać: statystyki');
		expect(screen.queryByRole('region', { name: 'Mój staż' })).not.toBeInTheDocument();
		expect(screen.getByRole('region', { name: 'Twoje etapy' })).toBeInTheDocument();
	});
});

const clubStats: NonNullable<DashboardStats['club']> = {
	total: seaTime({ voyage_count: 9, total_miles: 2775 }),
	by_year: []
};

describe('dashboard load', () => {
	it('loads the upcoming legs and sea time', async () => {
		const legs = [leg()];
		mockLegs.mockResolvedValue(legs);
		mockGet.mockResolvedValue(sailor);
		await expect(runLoad()).resolves.toEqual({ legs, stats: sailor, loadErrors: [] });
	});

	it.each([
		[
			'legs',
			() => mockLegs.mockRejectedValue(new Error('boom')),
			{ legs: null, stats: sailor },
			'Twoje etapy'
		],
		[
			'statistics',
			() => mockGet.mockRejectedValue(new Error('boom')),
			{ legs: [], stats: null },
			'statystyki'
		]
	])('marks failed %s instead of showing an empty section', async (_, fail, data, label) => {
		mockGet.mockResolvedValue(sailor);
		fail();
		await expect(runLoad()).resolves.toEqual({ ...data, loadErrors: [label] });
	});
});

describe('dashboard upcoming legs', () => {
	it('lists the legs above the statistics, each card linking to its leg', async () => {
		mockGet.mockResolvedValue(sailor);
		mockLegs.mockResolvedValue([
			leg({
				id: 7,
				name: 'Etap 1',
				series_name: 'Sycylia',
				edition: '2027',
				embark_date: '2027-05-01',
				disembark_date: '2027-05-08',
				start_port: { name: 'Palermo' },
				end_port: { name: 'Trapani' },
				relation: 'captain'
			}),
			leg({ id: 9, name: 'Bałtyk', cruise_name: 'Bałtyk 2027', relation: 'crew' }),
			leg({ id: 10, name: 'Mazury', relation: 'sign_up', sign_up_status: 'pending' }),
			leg({ id: 11, name: 'Chorwacja', relation: 'sign_up', sign_up_status: 'accepted' }),
			leg({ id: 12, name: 'Grecja', relation: 'sign_up', sign_up_status: 'waitlisted' })
		]);
		await renderPage();

		const mine = screen.getByRole('region', { name: 'Twoje etapy' });
		const stats = screen.getByRole('region', { name: 'Mój staż' });
		expect(mine.compareDocumentPosition(stats) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

		const cards = within(mine).getAllByRole('link');
		expect(cards.map((c) => c.getAttribute('href'))).toEqual([
			'/trips/7',
			'/trips/9',
			'/trips/10',
			'/trips/11',
			'/trips/12'
		]);
		expect(cards[0]).toHaveTextContent('Etap 1');
		expect(cards[0]).toHaveTextContent('Kapitan');
		expect(cards[0]).toHaveTextContent('Sycylia · edycja 2027');
		expect(cards[0]).toHaveTextContent('1–8 maj 2027 · 8 dni');
		expect(cards[0]).toHaveTextContent('Palermo → Trapani');
		expect(cards[1]).toHaveTextContent('W załodze');
		expect(cards[1]).toHaveTextContent('Bałtyk 2027');
		expect(cards[1]).toHaveTextContent('Termin do ustalenia');
		expect(cards[1]).not.toHaveTextContent('→');
		expect(cards[2]).toHaveTextContent('Zapis: oczekuje');
		expect(cards[3]).toHaveTextContent('Zapis: zaakceptowany');
		expect(cards[4]).toHaveTextContent('Zapis: lista rezerwowa');
	});

	it('lets a long unbroken leg or edition name wrap inside its card', async () => {
		const long = 'X'.repeat(200);
		mockGet.mockResolvedValue(sailor);
		mockLegs.mockResolvedValue([
			leg({ id: 7, name: long, cruise_name: long, relation: 'captain' })
		]);
		await renderPage();

		const mine = screen.getByRole('region', { name: 'Twoje etapy' });
		const card = within(mine).getByRole('link');
		expect(card.closest('li')).toHaveClass('min-w-0');
		expect(within(card).getByRole('heading', { name: long })).toHaveClass(
			'min-w-0',
			'[overflow-wrap:anywhere]'
		);
		expect(within(card).getByText(long, { selector: 'p' })).toHaveClass('[overflow-wrap:anywhere]');
		expect(within(card).getByText('Kapitan')).toHaveClass('shrink-0', 'whitespace-nowrap');
	});

	it('shows an empty state linking to the series list', async () => {
		mockGet.mockResolvedValue(sailor);
		await renderPage();

		const mine = screen.getByRole('region', { name: 'Twoje etapy' });
		expect(within(mine).getByText('Nie masz nadchodzących etapów.')).toBeInTheDocument();
		expect(within(mine).getByRole('link', { name: 'Zobacz rejsy' })).toHaveAttribute(
			'href',
			'/rejsy'
		);
	});

	it('names failed legs in the banner instead of an empty state', async () => {
		mockGet.mockResolvedValue(sailor);
		mockLegs.mockRejectedValue(new Error('Request failed: 500'));
		await renderPage();

		expect(screen.getByRole('alert')).toHaveTextContent('Nie udało się wczytać: Twoje etapy');
		expect(screen.queryByRole('region', { name: 'Twoje etapy' })).not.toBeInTheDocument();
		expect(screen.queryByText('Nie masz nadchodzących etapów.')).not.toBeInTheDocument();
		expect(screen.getByRole('region', { name: 'Mój staż' })).toBeInTheDocument();
	});
});

describe('dashboard club statistics', () => {
	it('hides club statistics from a member even when the reply has them', async () => {
		mockGet.mockResolvedValue({ ...sailor, club: clubStats });
		await renderPage();

		expect(screen.getByRole('region', { name: 'Mój staż' })).toBeInTheDocument();
		expect(screen.queryByRole('region', { name: 'Klub' })).not.toBeInTheDocument();
	});

	it('shows club statistics to an admin', async () => {
		mockAuth.isAdmin = true;
		mockGet.mockResolvedValue({ ...sailor, club: clubStats });
		await renderPage();

		expect(screen.getByRole('region', { name: 'Klub' })).toBeInTheDocument();
	});
});

describe('dashboard hidden legs', () => {
	const at = '2026-01-01T00:00:00Z';
	const cruiseHidden = 'Ukryte, bo wydarzenie jest ukryte';
	const draftReason = 'Szkic: widzą go tylko zarząd i kadra kapitańska';

	function mockHiddenRecords() {
		mockLegs.mockResolvedValue([
			leg({ id: 1, name: 'Szkic etapu', cruise_published_at: at }),
			leg({ id: 2, name: 'Etap ukrytego rejsu', published_at: at }),
			leg({ id: 3, name: 'Opublikowany', published_at: at, cruise_published_at: at })
		]);
		const voyage = { role: 'crew', miles: 10, hours_total: 2, days: 1 } as const;
		mockGet.mockResolvedValue({
			personal: {
				...sailor.personal,
				voyages: [
					{ ...voyage, id: 21, name: 'Szkic rejsu', cruise_published_at: at },
					{ ...voyage, id: 22, name: 'Rejs ukrytego wydarzenia', published_at: at },
					{
						...voyage,
						id: 23,
						name: 'Rejs opublikowany',
						published_at: at,
						cruise_published_at: at
					}
				]
			}
		});
	}

	function badgeReasons(region: string): (string | null)[] {
		return within(screen.getByRole('region', { name: region }))
			.getAllByRole('link')
			.map((link) => within(link).queryByTestId('hidden-badge')?.getAttribute('title') ?? null);
	}

	it.each([
		['admin', () => (mockAuth.isAdmin = true)],
		['captain group', () => (mockAuth.user.is_captain = true)]
	])('marks hidden legs and voyages for the %s', async (_, asViewer) => {
		asViewer();
		mockHiddenRecords();
		await renderPage();

		expect(badgeReasons('Twoje etapy')).toEqual([draftReason, cruiseHidden, null]);
		expect(badgeReasons('Mój staż')).toEqual([draftReason, cruiseHidden, null]);
	});

	it('shows no badge to a member', async () => {
		mockHiddenRecords();
		await renderPage();

		expect(badgeReasons('Twoje etapy')).toEqual([null, null, null]);
		expect(badgeReasons('Mój staż')).toEqual([null, null, null]);
	});
});
