import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/svelte';
import type { CruiseSeries, CruiseSeriesEdition } from '#lib/api/types.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/api/routes.js', () => ({
	getSeries: vi.fn(),
	listSeriesEditions: vi.fn(),
	updateSeries: vi.fn(),
	deleteSeries: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: false, user: { id: 2, role: 'member' } }
}));

import { goto } from '$app/navigation';
import { ApiError } from '#lib/api/client.js';
import { getSeries, listSeriesEditions, updateSeries, deleteSeries } from '#lib/api/routes.js';
import { auth } from '#lib/stores/auth.svelte.js';
import SeriesDetailPage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

function runLoad(id = '4') {
	return load({ params: { id } } as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

// renderPage runs the route's load function and renders the page with its
// data, the way SvelteKit does on navigation.
async function renderPage() {
	return render(SeriesDetailPage, { props: { data: await runLoad() } });
}

const mockUpdate = updateSeries as unknown as ReturnType<typeof vi.fn>;
const mockDelete = deleteSeries as unknown as ReturnType<typeof vi.fn>;
const mockAuth = auth as unknown as { isAdmin: boolean };

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
	mockAuth.isAdmin = false;
	mockUpdate.mockReset().mockResolvedValue(undefined);
	mockDelete.mockReset().mockResolvedValue(undefined);
	vi.mocked(goto).mockReset();
	mockGet.mockReset().mockResolvedValue(series);
	mockEditions.mockReset().mockResolvedValue([]);
	vi.spyOn(console, 'error').mockImplementation(() => {});
});

describe('series detail page', () => {
	it('loads the series and its editions by id', async () => {
		await renderPage();
		expect(await screen.findByText('May I Sail')).toBeInTheDocument();
		expect(mockGet).toHaveBeenCalledWith(4);
		expect(mockEditions).toHaveBeenCalledWith(4);
		expect(screen.getByText(/Brak edycji/)).toBeInTheDocument();
	});

	it('lists editions in API order with per-edition and overall totals', async () => {
		mockEditions.mockResolvedValue([
			makeEdition({
				cruise_id: 20,
				name: 'MIS 2025',
				edition: '2025',
				total_miles: 200,
				total_hours: 40
			}),
			makeEdition({
				cruise_id: 10,
				name: 'MIS 2024',
				edition: '2024',
				kind: 'inland',
				total_miles: 100.5,
				total_hours: 20
			})
		]);
		await renderPage();
		await screen.findByText('May I Sail');

		const rows = screen.getAllByTestId('edition-row');
		expect(rows).toHaveLength(2);
		expect(within(rows[0]).getByText('2025')).toBeInTheDocument();
		expect(within(rows[0]).getByRole('link', { name: 'MIS 2025' })).toHaveAttribute(
			'href',
			'/cruises/20'
		);
		expect(within(rows[1]).getByText('Śródlądowy')).toBeInTheDocument();
		expect(within(rows[1]).getByText('100,5')).toBeInTheDocument();

		const totals = screen.getByTestId('series-totals');
		expect(within(totals).getByText('300,5')).toBeInTheDocument();
		expect(within(totals).getByText('60')).toBeInTheDocument();
	});

	it('hides edit and delete for a member', async () => {
		await renderPage();
		await screen.findByText('May I Sail');
		expect(screen.queryByRole('button', { name: 'Edytuj' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Usuń' })).not.toBeInTheDocument();
	});

	it('throws a 404 with the series copy for +error.svelte', async () => {
		mockGet.mockRejectedValue(new ApiError(404, 'series not found'));
		await expect(runLoad()).rejects.toMatchObject({
			status: 404,
			body: {
				message: 'Ta seria nie istnieje lub nie masz do niej dostępu.',
				title: 'Nie znaleziono serii',
				backHref: '/series'
			}
		});
	});

	it('throws the series 404 when the editions 404 arrives first', async () => {
		mockGet.mockImplementation(
			() =>
				new Promise((_, reject) =>
					setTimeout(() => reject(new ApiError(404, 'series not found')), 10)
				)
		);
		mockEditions.mockRejectedValue(new ApiError(404, 'series not found'));
		await expect(runLoad()).rejects.toMatchObject({
			status: 404,
			body: {
				message: 'Ta seria nie istnieje lub nie masz do niej dostępu.',
				title: 'Nie znaleziono serii'
			}
		});
	});

	it('passes other failures on to +error.svelte', async () => {
		const boom = new Error('network');
		mockGet.mockRejectedValue(boom);
		await expect(runLoad()).rejects.toBe(boom);
	});

	it('lets an admin rename the series and clear its description', async () => {
		mockAuth.isAdmin = true;
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Edytuj' }));

		expect(screen.getByLabelText('Opis')).toHaveValue('Od 2018');
		await fireEvent.input(screen.getByLabelText('Nazwa *'), { target: { value: 'MIS' } });
		await fireEvent.input(screen.getByLabelText('Opis'), { target: { value: '' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz zmiany' }));

		expect(mockUpdate).toHaveBeenCalledWith(4, { name: 'MIS', description: undefined });
		expect(await screen.findByRole('heading', { name: 'MIS' })).toBeInTheDocument();
		expect(screen.queryByText('Od 2018')).not.toBeInTheDocument();
	});

	it('keeps the form open with the error when saving fails', async () => {
		mockAuth.isAdmin = true;
		mockUpdate.mockRejectedValue(new Error('name taken'));
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Edytuj' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz zmiany' }));

		expect(await screen.findByText('name taken')).toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Anuluj' }));
		expect(screen.getByRole('heading', { name: 'May I Sail' })).toBeInTheDocument();
	});

	it('deletes the series after confirmation', async () => {
		mockAuth.isAdmin = true;
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));

		expect(confirmSpy).toHaveBeenCalled();
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/series'));
		expect(mockDelete).toHaveBeenCalledWith(4);
	});

	it('keeps the series when deletion is not confirmed', async () => {
		mockAuth.isAdmin = true;
		vi.spyOn(window, 'confirm').mockReturnValue(false);
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));
		expect(mockDelete).not.toHaveBeenCalled();
	});
});

describe('series page same-route navigation', () => {
	const other: CruiseSeries = { ...series, id: 5, name: 'Baltic Cup', description: undefined };

	function loadBoth() {
		mockGet.mockImplementation(async (id: number) => (id === 5 ? other : series));
		mockEditions.mockImplementation(async (id: number) =>
			id === 5 ? [makeEdition({ cruise_id: 50, name: 'Baltic Cup 2025', total_miles: 80 })] : []
		);
	}

	it('shows the second series after a link moves to it', async () => {
		loadBoth();
		const view = await renderPage();
		expect(await screen.findByText('May I Sail')).toBeInTheDocument();

		await view.rerender({ data: await runLoad('5') });

		expect(screen.getByRole('heading', { name: 'Baltic Cup' })).toBeInTheDocument();
		expect(screen.queryByText('May I Sail')).not.toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'Baltic Cup 2025' })).toHaveAttribute(
			'href',
			'/cruises/50'
		);
	});

	it('closes the edit form of the previous series', async () => {
		loadBoth();
		mockAuth.isAdmin = true;
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Edytuj' }));
		expect(screen.getByLabelText('Nazwa *')).toBeInTheDocument();

		await view.rerender({ data: await runLoad('5') });

		expect(screen.queryByLabelText('Nazwa *')).not.toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'Baltic Cup' })).toBeInTheDocument();
	});

	it('drops a rename reply for a series the page has left', async () => {
		loadBoth();
		mockAuth.isAdmin = true;
		let finish: () => void = () => {};
		mockUpdate.mockImplementation(() => new Promise<void>((r) => (finish = r)));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Edytuj' }));
		await fireEvent.input(screen.getByLabelText('Nazwa *'), { target: { value: 'Renamed' } });
		await fireEvent.click(screen.getByRole('button', { name: /Zapisz/ }));

		await view.rerender({ data: await runLoad('5') });
		finish();
		expect(mockUpdate).toHaveBeenCalledWith(4, expect.anything());
		await new Promise((r) => setTimeout(r, 0));

		expect(screen.getByRole('heading', { name: 'Baltic Cup' })).toBeInTheDocument();
		expect(screen.queryByText('Renamed')).not.toBeInTheDocument();
	});

	it('clears the busy flag of a save left pending on the previous series', async () => {
		loadBoth();
		mockAuth.isAdmin = true;
		mockUpdate.mockImplementation(() => new Promise<void>(() => {}));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Edytuj' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz zmiany' }));
		expect(screen.getByRole('button', { name: 'Zapisywanie...' })).toBeDisabled();

		await view.rerender({ data: await runLoad('5') });
		await fireEvent.click(screen.getByRole('button', { name: 'Edytuj' }));

		expect(screen.getByRole('button', { name: 'Zapisz zmiany' })).toBeEnabled();
	});
});
