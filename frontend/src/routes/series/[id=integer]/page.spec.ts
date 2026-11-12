import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/svelte';
import type { CruiseSeries, CruiseSeriesEdition } from '#lib/api/types.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: { params: { id: '4' } } }));
vi.mock('#lib/api/routes.js', () => ({
	getSeries: vi.fn(),
	listSeriesEditions: vi.fn(),
	updateSeries: vi.fn(),
	deleteSeries: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: false, user: { id: 2, role: 'member' } }
}));

import { ApiError } from '#lib/api/client.js';
import { getSeries, listSeriesEditions } from '#lib/api/routes.js';
import SeriesDetailPage from './+page.svelte';

const mockGet = getSeries as unknown as ReturnType<typeof vi.fn>;
const mockEditions = listSeriesEditions as unknown as ReturnType<typeof vi.fn>;

const series: CruiseSeries = {
	id: 4,
	name: 'May I Sail',
	description: 'Od 2018',
	created_at: '2026-01-01T00:00:00Z',
	updated_at: '2026-01-01T00:00:00Z'
};

function makeEdition(overrides: Partial<CruiseSeriesEdition> = {}): CruiseSeriesEdition {
	return {
		cruise_id: 1,
		name: 'MIS',
		kind: 'sea',
		voyage_count: 0,
		total_miles: 0,
		total_hours: 0,
		...overrides
	};
}

beforeEach(() => {
	mockGet.mockReset().mockResolvedValue(series);
	mockEditions.mockReset().mockResolvedValue([]);
	vi.spyOn(console, 'error').mockImplementation(() => {});
});

describe('series detail page', () => {
	it('loads the series and its editions by id', async () => {
		render(SeriesDetailPage);
		expect(await screen.findByText('May I Sail')).toBeInTheDocument();
		expect(mockGet).toHaveBeenCalledWith(4);
		expect(mockEditions).toHaveBeenCalledWith(4);
		expect(screen.getByText(/Brak edycji/)).toBeInTheDocument();
	});

	it('lists editions in API order with per-edition and overall totals', async () => {
		mockEditions.mockResolvedValue([
			makeEdition({ cruise_id: 20, name: 'MIS 2025', edition: '2025', total_miles: 200, total_hours: 40 }),
			makeEdition({ cruise_id: 10, name: 'MIS 2024', edition: '2024', kind: 'inland', total_miles: 100.5, total_hours: 20 })
		]);
		render(SeriesDetailPage);
		await screen.findByText('May I Sail');

		const rows = screen.getAllByTestId('edition-row');
		expect(rows).toHaveLength(2);
		expect(within(rows[0]).getByText('2025')).toBeInTheDocument();
		expect(within(rows[0]).getByRole('link', { name: 'MIS 2025' })).toHaveAttribute('href', '/cruises/20');
		expect(within(rows[1]).getByText('Śródlądowy')).toBeInTheDocument();
		expect(within(rows[1]).getByText('100,5')).toBeInTheDocument();

		const totals = screen.getByTestId('series-totals');
		expect(within(totals).getByText('300,5')).toBeInTheDocument();
		expect(within(totals).getByText('60')).toBeInTheDocument();
	});

	it('hides edit and delete for a member', async () => {
		render(SeriesDetailPage);
		await screen.findByText('May I Sail');
		expect(screen.queryByRole('button', { name: 'Edytuj' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Usuń' })).not.toBeInTheDocument();
	});

	it('shows not found for an unknown series', async () => {
		mockGet.mockRejectedValue(new ApiError(404, 'series not found'));
		render(SeriesDetailPage);
		expect(await screen.findByText('Nie znaleziono serii')).toBeInTheDocument();
	});
});
