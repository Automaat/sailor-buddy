import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { makeCruise } from '#lib/test-utils.js';
import { ApiError } from '#lib/api/client.js';
import type { CruiseSeries } from '#lib/api/types.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));
vi.mock('#lib/api/routes.js', () => ({
	getCruise: vi.fn(),
	updateCruise: vi.fn(),
	listSeries: vi.fn()
}));

import { getCruise, updateCruise, listSeries } from '#lib/api/routes.js';
import { goto } from '$app/navigation';
import EditCruisePage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

const fn = (f: unknown) => f as ReturnType<typeof vi.fn>;

function runLoad(id = '7') {
	return load({ params: { id } } as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

/** renderPage runs the route's load function and renders the page with its data, the way SvelteKit does on navigation. */
async function renderPage(id = '7') {
	return render(EditCruisePage, { props: { data: await runLoad(id) } });
}

async function save() {
	await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));
	await waitFor(() => expect(updateCruise).toHaveBeenCalled());
	return fn(updateCruise).mock.calls[0];
}

function seriesRow(id: number, name: string): CruiseSeries {
	return { id, name, created_at: '', updated_at: '' } as CruiseSeries;
}

beforeEach(() => {
	vi.clearAllMocks();
	fn(getCruise).mockImplementation(async (id: number) =>
		id === 7
			? makeCruise({
					id: 7,
					name: 'Sycylia 2026',
					series_id: 3,
					edition: '2026',
					included: ['Jacht']
				})
			: makeCruise({ id, name: 'Chorwacja 2026', start_port: 'Split' })
	);
	fn(listSeries).mockResolvedValue([seriesRow(3, 'Sycylia'), seriesRow(4, 'Chorwacja')]);
	fn(updateCruise).mockResolvedValue(undefined);
});

describe('cruise edit page', () => {
	it('fills the form from the loaded cruise and series list', async () => {
		await renderPage();

		expect(screen.getByLabelText('Nazwa wydarzenia *')).toHaveValue('Sycylia 2026');
		expect(screen.getByLabelText('Seria')).toHaveValue('3');
		expect(screen.getByRole('option', { name: 'Chorwacja' })).toBeInTheDocument();
		expect(listSeries).toHaveBeenCalledTimes(1);
	});

	it('keeps the series and included items on a no-change save', async () => {
		await renderPage();

		const [id, body] = await save();

		expect(id).toBe(7);
		expect(body).toMatchObject({
			name: 'Sycylia 2026',
			series_id: 3,
			edition: '2026',
			included: ['Jacht']
		});
		expect(goto).toHaveBeenCalledWith('/cruises/7');
	});

	it('shows the server error when saving fails', async () => {
		fn(updateCruise).mockRejectedValue(new Error('boom'));
		await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz zmiany' }));

		expect(await screen.findByText('boom')).toBeInTheDocument();
		expect(goto).not.toHaveBeenCalled();
	});
});

describe('cruise edit page load', () => {
	it('loads the cruise named by the URL', async () => {
		const data = await runLoad('9');

		expect(getCruise).toHaveBeenCalledWith(9);
		expect(data.cruise.id).toBe(9);
	});

	it('throws a 404 with the cruise copy for +error.svelte', async () => {
		fn(getCruise).mockRejectedValue(new ApiError(404, 'not found'));

		await expect(runLoad()).rejects.toMatchObject({
			status: 404,
			body: {
				message: 'To wydarzenie nie istnieje lub nie masz do niego dostępu.',
				title: 'Nie znaleziono wydarzenia',
				backHref: '/cruises'
			}
		});
	});

	it('throws a 404 when the API returns no cruise', async () => {
		fn(getCruise).mockResolvedValue(null);

		await expect(runLoad()).rejects.toMatchObject({ status: 404 });
	});

	it('passes other cruise failures on to +error.svelte', async () => {
		const boom = new Error('boom');
		fn(getCruise).mockRejectedValue(boom);

		await expect(runLoad()).rejects.toBe(boom);
	});

	it('fails the load when the series list fails, so a save cannot unlink the series', async () => {
		const boom = new Error('series down');
		fn(listSeries).mockRejectedValue(boom);

		await expect(runLoad()).rejects.toBe(boom);
	});
});

describe('cruise edit page same-route navigation', () => {
	it('shows the second cruise and drops edits made to the first', async () => {
		const view = await renderPage();
		await fireEvent.input(screen.getByLabelText('Nazwa wydarzenia *'), {
			target: { value: 'Zmienione' }
		});

		await view.rerender({ data: await runLoad('8') });

		expect(screen.getByLabelText('Nazwa wydarzenia *')).toHaveValue('Chorwacja 2026');
		expect(screen.getByLabelText('Seria')).toHaveValue('');
		expect(screen.getByRole('link', { name: 'Anuluj' })).toHaveAttribute('href', '/cruises/8');
		const [id, body] = await save();
		expect(id).toBe(8);
		expect(body).toMatchObject({ name: 'Chorwacja 2026', start_port: 'Split', included: [] });
	});

	it("clears the first cruise's save error", async () => {
		fn(updateCruise).mockRejectedValueOnce(new Error('boom'));
		const view = await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz zmiany' }));
		expect(await screen.findByText('boom')).toBeInTheDocument();

		await view.rerender({ data: await runLoad('8') });

		expect(screen.queryByText('boom')).not.toBeInTheDocument();
	});

	it('drops a save reply for the cruise the page has left', async () => {
		let answer: (v?: unknown) => void = () => {};
		fn(updateCruise).mockReturnValueOnce(new Promise((resolve) => (answer = resolve)));
		const view = await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz zmiany' }));
		expect(screen.getByRole('button', { name: 'Zapisywanie...' })).toBeDisabled();

		await view.rerender({ data: await runLoad('8') });

		expect(screen.getByRole('button', { name: 'Zapisz zmiany' })).toBeEnabled();
		answer();
		await new Promise((r) => setTimeout(r, 0));
		expect(goto).not.toHaveBeenCalled();
	});
});
