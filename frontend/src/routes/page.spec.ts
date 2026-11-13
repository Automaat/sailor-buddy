import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/svelte';
import type { DashboardStats, SeaTime } from '#lib/api/types.js';

vi.mock('#lib/api/routes.js', () => ({ getDashboard: vi.fn() }));

import { getDashboard } from '#lib/api/routes.js';
import DashboardPage from './+page.svelte';

const mockGet = getDashboard as unknown as ReturnType<typeof vi.fn>;

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
	vi.spyOn(console, 'error').mockImplementation(() => {});
});

describe('dashboard page', () => {
	it('shows a loading message while data is being fetched', () => {
		mockGet.mockReturnValue(new Promise(() => {}));
		render(DashboardPage);
		expect(screen.getByText('Wczytywanie...')).toBeInTheDocument();
	});

	it("shows the user's own totals, role split, years and voyages", async () => {
		mockGet.mockResolvedValue(sailor);
		render(DashboardPage);

		const mine = await screen.findByRole('region', { name: 'Mój staż' });
		expect(within(mine).getByText('Zrealizowane rejsy').nextElementSibling).toHaveTextContent('2');
		expect(within(mine).getByText('Mile morskie').nextElementSibling).toHaveTextContent('320');
		expect(within(mine).getByText('60h żagle / 20h silnik')).toBeInTheDocument();
		expect(within(mine).getByText('Jako kapitan').nextElementSibling).toHaveTextContent(
			'1 rejsy · 200 Mm · 50 h · 6 dni'
		);
		expect(within(mine).getByText('Jako załoga').nextElementSibling).toHaveTextContent(
			'1 rejsy · 120 Mm · 30 h · 4 dni'
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

	it('hides the club section when the response has none (member)', async () => {
		mockGet.mockResolvedValue(sailor);
		render(DashboardPage);
		await screen.findByRole('region', { name: 'Mój staż' });
		expect(screen.queryByRole('region', { name: 'Klub' })).not.toBeInTheDocument();
	});

	it('shows labelled club totals for an admin', async () => {
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
		render(DashboardPage);

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
		render(DashboardPage);

		const mine = await screen.findByRole('region', { name: 'Mój staż' });
		expect(
			within(mine).getByText('Nie masz jeszcze zrealizowanych rejsów z klubem.')
		).toBeInTheDocument();
		expect(within(mine).getByRole('link', { name: 'Zobacz rejsy klubu' })).toHaveAttribute(
			'href',
			'/cruises'
		);
		expect(within(mine).queryByText('Zrealizowane rejsy')).not.toBeInTheDocument();
	});

	it('shows an error message when loading fails', async () => {
		mockGet.mockRejectedValue(new Error('Request failed: 500'));
		render(DashboardPage);

		expect(await screen.findByRole('alert')).toHaveTextContent('Nie udało się wczytać pulpitu');
		expect(screen.queryByRole('region', { name: 'Mój staż' })).not.toBeInTheDocument();
	});
});
