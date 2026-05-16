import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import { makeTrip } from '$lib/test-utils';

vi.mock('$lib/api/client', () => ({ api: { list: vi.fn() } }));
vi.mock('$lib/stores/org.svelte', () => ({
	orgStore: { currentSlug: 'alfa', apiPrefix: () => '/orgs/alfa' }
}));

import { api } from '$lib/api/client';
import TripsPage from './+page.svelte';

const apiList = api.list as unknown as ReturnType<typeof vi.fn>;

beforeEach(() => {
	apiList.mockReset().mockResolvedValue([]);
	vi.spyOn(console, 'error').mockImplementation(() => {});
});

describe('trips page', () => {
	it('shows a loading message while trips are being fetched', () => {
		apiList.mockReturnValue(new Promise(() => {}));
		render(TripsPage);
		expect(screen.getByText('Wczytywanie...')).toBeInTheDocument();
	});

	it('requests trips from the org-scoped endpoint', async () => {
		render(TripsPage);
		await screen.findByText('Brak planowanych rejsów');
		expect(apiList).toHaveBeenCalledWith('/orgs/alfa/trips');
	});

	it('shows the empty state when there are no trips', async () => {
		render(TripsPage);
		expect(await screen.findByText('Brak planowanych rejsów')).toBeInTheDocument();
		expect(screen.getByText('Zaplanuj pierwszy rejs')).toBeInTheDocument();
	});

	it('renders trip cards with route, dates and crew details', async () => {
		apiList.mockResolvedValue([
			makeTrip({
				id: 7,
				name: 'Rejs Bałtyk',
				start_port: 'Gdynia',
				end_port: 'Hel',
				countries: 'Polska',
				captain_name: 'Jan Kowalski',
				max_crew: 6
			})
		]);
		render(TripsPage);

		const card = (await screen.findByText('Rejs Bałtyk')).closest('a');
		expect(card).toHaveAttribute('href', '/trips/7');
		expect(screen.getByText(/Gdynia/)).toBeInTheDocument();
		expect(screen.getByText('kpt. Jan Kowalski')).toBeInTheDocument();
		expect(screen.getByText('max 6 os.')).toBeInTheDocument();
	});

	it('labels a planned trip', async () => {
		apiList.mockResolvedValue([makeTrip({ status: 'planned' })]);
		render(TripsPage);
		expect(await screen.findByText('Planowany')).toBeInTheDocument();
	});

	it('labels a cancelled trip', async () => {
		apiList.mockResolvedValue([makeTrip({ status: 'cancelled' })]);
		render(TripsPage);
		expect(await screen.findByText('Anulowany')).toBeInTheDocument();
	});

	it('falls back to the empty state when loading fails', async () => {
		apiList.mockRejectedValue(new Error('network down'));
		render(TripsPage);
		expect(await screen.findByText('Brak planowanych rejsów')).toBeInTheDocument();
		expect(console.error).toHaveBeenCalled();
	});
});
