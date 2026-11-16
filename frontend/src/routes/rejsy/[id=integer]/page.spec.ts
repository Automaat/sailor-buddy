import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/svelte';
import type { CruiseSeries, CruiseSeriesEdition } from '#lib/api/types.js';
import { makeSeries } from '#lib/test-utils.js';

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

const series = makeSeries({ id: 4, name: 'May I Sail', description: 'Od 2018', kind: 'sea' });

function makeEdition(overrides: Partial<CruiseSeriesEdition> = {}): CruiseSeriesEdition {
	return {
		cruise_id: 1,
		name: 'MIS',
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
	it('links back to the Rejsy list above the title', async () => {
		await renderPage();
		expect(screen.getByTestId('back-link')).toHaveAttribute('href', '/rejsy');
		expect(screen.getByRole('link', { name: 'Rejsy' })).toBe(screen.getByTestId('back-link'));
	});

	it('loads the series and its editions by id', async () => {
		await renderPage();
		expect(await screen.findByText('May I Sail')).toBeInTheDocument();
		expect(mockGet).toHaveBeenCalledWith(4);
		expect(mockEditions).toHaveBeenCalledWith(4);
		expect(screen.getByText(/Brak edycji/)).toBeInTheDocument();
	});

	it.each([
		['sea', 'Morski'],
		['inland', 'Śródlądowy']
	] as const)('shows the %s kind of the series under its name', async (kind, label) => {
		mockGet.mockResolvedValue({ ...series, kind });
		await renderPage();
		expect(await screen.findByText(label)).toBeInTheDocument();
	});

	it('lists editions in API order with only their label and name', async () => {
		mockEditions.mockResolvedValue([
			makeEdition({ cruise_id: 20, name: 'MIS 2025', edition: '2025' }),
			makeEdition({ cruise_id: 10, name: 'MIS 2024', edition: '2024' }),
			makeEdition({ cruise_id: 5, name: 'MIS bez etykiety' })
		]);
		await renderPage();
		await screen.findByText('May I Sail');

		expect(screen.getAllByRole('columnheader').map((h) => h.textContent?.trim())).toEqual([
			'Edycja',
			'Nazwa'
		]);
		const rows = screen.getAllByTestId('edition-row');
		expect(rows).toHaveLength(3);
		for (const row of rows) expect(within(row).getAllByRole('cell')).toHaveLength(2);
		expect(within(rows[0]).getByText('2025')).toBeInTheDocument();
		expect(within(rows[0]).getByRole('link', { name: 'MIS 2025' })).toHaveAttribute(
			'href',
			'/cruises/20'
		);
		expect(within(rows[1]).getByRole('link', { name: 'MIS 2024' })).toHaveAttribute(
			'href',
			'/cruises/10'
		);
		expect(within(rows[2]).getByText('—')).toBeInTheDocument();
		expect(screen.queryByText(/Mile|Godziny|Razem/)).not.toBeInTheDocument();
		// A long name without spaces must wrap instead of widening the table on a phone.
		expect(screen.getByRole('table')).toHaveClass('table-fixed');
		expect(within(rows[0]).getByRole('link')).toHaveClass('[overflow-wrap:anywhere]');
	});

	it('hides edit, delete and new edition for a member', async () => {
		await renderPage();
		await screen.findByText('May I Sail');
		expect(screen.queryByRole('button', { name: 'Edytuj' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Usuń' })).not.toBeInTheDocument();
		expect(screen.queryByRole('link', { name: '+ Nowa edycja' })).not.toBeInTheDocument();
		expect(screen.getByText('Brak edycji.')).toBeInTheDocument();
	});

	it('links an admin to a new edition of this series', async () => {
		mockAuth.isAdmin = true;
		await renderPage();
		await screen.findByText('May I Sail');
		expect(screen.getByRole('link', { name: '+ Nowa edycja' })).toHaveAttribute(
			'href',
			'/cruises/new?series_id=4'
		);
		expect(screen.getByText(/Dodaj pierwszą przyciskiem „\+ Nowa edycja”/)).toBeInTheDocument();
	});

	it('throws a 404 with the series copy for +error.svelte', async () => {
		mockGet.mockRejectedValue(new ApiError(404, 'series not found'));
		await expect(runLoad()).rejects.toMatchObject({
			status: 404,
			body: {
				message: 'Ta seria nie istnieje lub nie masz do niej dostępu.',
				title: 'Nie znaleziono serii',
				backHref: '/rejsy'
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

		expect(mockUpdate).toHaveBeenCalledWith(4, {
			name: 'MIS',
			description: undefined,
			kind: 'sea'
		});
		expect(await screen.findByRole('heading', { name: 'MIS' })).toBeInTheDocument();
		expect(screen.queryByText('Od 2018')).not.toBeInTheDocument();
	});

	it("lets an admin change the series' kind", async () => {
		mockAuth.isAdmin = true;
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Edytuj' }));

		expect(screen.getByLabelText('Rodzaj rejsów')).toHaveValue('sea');
		await fireEvent.change(screen.getByLabelText('Rodzaj rejsów'), {
			target: { value: 'inland' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz zmiany' }));

		expect(mockUpdate).toHaveBeenCalledWith(4, {
			name: 'May I Sail',
			description: 'Od 2018',
			kind: 'inland'
		});
		expect(await screen.findByText('Śródlądowy')).toBeInTheDocument();
		expect(screen.queryByText('Morski')).not.toBeInTheDocument();
	});

	it('lets a long unbroken series name and description wrap', async () => {
		const long = 'X'.repeat(200);
		mockGet.mockResolvedValue({ ...series, name: long, description: long });
		await renderPage();

		const heading = await screen.findByRole('heading', { name: long });
		expect(heading).toHaveClass('[overflow-wrap:anywhere]');
		expect(heading.parentElement).toHaveClass('min-w-0');
		expect(screen.getByText(long, { selector: 'p' })).toHaveClass('[overflow-wrap:anywhere]');
	});

	it('keeps a long unbroken edition label inside its cell', async () => {
		const label = 'E' + 'W'.repeat(49);
		mockEditions.mockResolvedValue([
			makeEdition({ cruise_id: 7, name: 'Normal edition', edition: label })
		]);
		await renderPage();

		const badge = await screen.findByText(label);
		expect(badge).toHaveClass('max-w-full', '[overflow-wrap:anywhere]');
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
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/rejsy'));
		expect(mockDelete).toHaveBeenCalledWith(4);
	});

	it('shows the error and stays on the series when deletion fails', async () => {
		mockAuth.isAdmin = true;
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		mockDelete.mockRejectedValue(new ApiError(409, 'series is in use'));
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));

		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Nie udało się usunąć serii: series is in use'
		);
		expect(screen.getByRole('heading', { name: 'May I Sail' })).toBeInTheDocument();
		expect(goto).not.toHaveBeenCalled();
	});

	it('clears a delete error when a retry succeeds', async () => {
		mockAuth.isAdmin = true;
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		mockDelete.mockRejectedValueOnce(new Error('offline'));
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));
		expect(await screen.findByRole('alert')).toHaveTextContent('offline');

		await fireEvent.click(screen.getByRole('button', { name: 'Usuń' }));
		await vi.waitFor(() => expect(goto).toHaveBeenCalledWith('/rejsy'));
		expect(screen.queryByRole('alert')).not.toBeInTheDocument();
	});

	it('clears a delete error when the edit form opens', async () => {
		mockAuth.isAdmin = true;
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		mockDelete.mockRejectedValue(new ApiError(409, 'series is in use'));
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));
		expect(await screen.findByRole('alert')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Edytuj' }));

		expect(screen.getByLabelText('Nazwa *')).toBeInTheDocument();
		expect(screen.queryByRole('alert')).not.toBeInTheDocument();
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
			id === 5 ? [makeEdition({ cruise_id: 50, name: 'Baltic Cup 2025' })] : []
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

	it('drops a delete failure that answers after a link moved to another series', async () => {
		loadBoth();
		mockAuth.isAdmin = true;
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		let fail: (err: Error) => void = () => {};
		mockDelete.mockImplementation(() => new Promise<void>((_, reject) => (fail = reject)));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));

		await view.rerender({ data: await runLoad('5') });
		fail(new ApiError(409, 'series is in use'));
		expect(mockDelete).toHaveBeenCalledWith(4);
		await new Promise((r) => setTimeout(r, 0));

		expect(screen.getByRole('heading', { name: 'Baltic Cup' })).toBeInTheDocument();
		expect(screen.queryByRole('alert')).not.toBeInTheDocument();
	});

	it('stays on the new series when a delete succeeds after a link moved to it', async () => {
		loadBoth();
		mockAuth.isAdmin = true;
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		let finish: () => void = () => {};
		mockDelete.mockImplementation(() => new Promise<void>((r) => (finish = r)));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));

		await view.rerender({ data: await runLoad('5') });
		finish();
		await new Promise((r) => setTimeout(r, 0));

		expect(goto).not.toHaveBeenCalled();
		expect(screen.getByRole('heading', { name: 'Baltic Cup' })).toBeInTheDocument();
	});
});

describe('series detail page hidden editions', () => {
	const authState = auth as unknown as { isAdmin: boolean; user: Record<string, unknown> };
	const row = (name: string) =>
		screen.getAllByTestId('edition-row').find((r) => within(r).queryByText(name))!;

	beforeEach(() => {
		mockEditions.mockResolvedValue([
			makeEdition({ cruise_id: 1, name: 'MIS 2025', published_at: '2025-01-01T00:00:00Z' }),
			makeEdition({ cruise_id: 2, name: 'MIS 2026' })
		]);
	});

	afterEach(() => {
		authState.isAdmin = false;
		authState.user = { id: 2, role: 'member' };
	});

	it.each([
		['an admin', { isAdmin: true, user: { id: 1, role: 'admin', is_captain: false } }],
		['the captain group', { isAdmin: false, user: { id: 3, role: 'member', is_captain: true } }]
	])('marks hidden editions for %s', async (_who, state) => {
		Object.assign(authState, state);
		await renderPage();
		expect(await within(row('MIS 2026')).findByText('Ukryte')).toBeInTheDocument();
		expect(within(row('MIS 2025')).queryByText('Ukryte')).not.toBeInTheDocument();
	});

	it('shows a member no badge', async () => {
		authState.user = { id: 2, role: 'member', is_captain: false };
		await renderPage();
		expect(await screen.findByText('MIS 2026')).toBeInTheDocument();
		expect(screen.queryByText('Ukryte')).not.toBeInTheDocument();
	});
});
