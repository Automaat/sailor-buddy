import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import { makeVoyage } from '$lib/test-utils';

vi.mock('$lib/api/client', () => ({ api: { list: vi.fn() } }));
vi.mock('$lib/stores/org.svelte', () => ({
	orgStore: { currentSlug: 'alfa', isOrgAdmin: true, apiPrefix: () => '/orgs/alfa' }
}));

import { api } from '$lib/api/client';
import VoyagesPage from './+page.svelte';

const apiList = api.list as unknown as ReturnType<typeof vi.fn>;

beforeEach(() => {
	apiList.mockReset().mockResolvedValue([]);
	vi.spyOn(console, 'error').mockImplementation(() => {});
});

describe('voyages page', () => {
	it('shows a loading message while voyages are being fetched', () => {
		apiList.mockReturnValue(new Promise(() => {}));
		render(VoyagesPage);
		expect(screen.getByText('Wczytywanie...')).toBeInTheDocument();
	});

	it('requests voyages from the org-scoped endpoint', async () => {
		render(VoyagesPage);
		await screen.findByText('Brak zrealizowanych rejsów');
		expect(apiList).toHaveBeenCalledWith('/orgs/{slug}/voyages', { path: { slug: 'alfa' } });
	});

	it('shows the empty state when there are no voyages', async () => {
		render(VoyagesPage);
		expect(await screen.findByText('Brak zrealizowanych rejsów')).toBeInTheDocument();
		expect(screen.getByText('Wpisz pierwszy rejs')).toBeInTheDocument();
	});

	it('renders voyage cards with rounded hour and mile stats', async () => {
		apiList.mockResolvedValue([
			makeVoyage({
				id: 4,
				name: 'Rejs Skandynawia',
				hours_total: 42.6,
				miles: 130.4,
				days: 7
			})
		]);
		render(VoyagesPage);

		const card = (await screen.findByText('Rejs Skandynawia')).closest('a');
		expect(card).toHaveAttribute('href', '/voyages/4');
		expect(screen.getByText('43h')).toBeInTheDocument();
		expect(screen.getByText('130')).toBeInTheDocument();
		expect(screen.getByText('7')).toBeInTheDocument();
	});

	it('falls back to the empty state when loading fails', async () => {
		apiList.mockRejectedValue(new Error('network down'));
		render(VoyagesPage);
		expect(await screen.findByText('Brak zrealizowanych rejsów')).toBeInTheDocument();
		expect(console.error).toHaveBeenCalled();
	});
});
