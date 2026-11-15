import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/api/routes.js', () => ({ createCruise: vi.fn(), getSeries: vi.fn() }));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));

import { goto } from '$app/navigation';
import { ApiError } from '#lib/api/client.js';
import { createCruise, getSeries } from '#lib/api/routes.js';
import { makeSeries } from '#lib/test-utils.js';
import NewEditionPage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

const mockCreate = createCruise as unknown as ReturnType<typeof vi.fn>;
const mockGetSeries = getSeries as unknown as ReturnType<typeof vi.fn>;

const series = makeSeries({ id: 3, name: 'May I Sail' });

function runLoad(href = 'http://localhost/cruises/new?series_id=3') {
	return load({ url: new URL(href) } as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

/** renderPage runs the route's load function and renders the page with its data, the way SvelteKit does on navigation. */
async function renderPage(href?: string) {
	return render(NewEditionPage, { props: { data: await runLoad(href) } });
}

async function fillRequired(name: string, edition: string) {
	await fireEvent.input(screen.getByLabelText('Nazwa wydarzenia *'), { target: { value: name } });
	await fireEvent.input(screen.getByLabelText('Edycja *'), { target: { value: edition } });
}

beforeEach(() => {
	mockCreate.mockReset().mockResolvedValue({ id: 5 });
	mockGetSeries.mockReset().mockResolvedValue(series);
	vi.mocked(goto).mockReset();
});

describe('new edition page load', () => {
	it('loads the series named by ?series_id=', async () => {
		const data = await runLoad();
		expect(mockGetSeries).toHaveBeenCalledWith(3);
		expect(data.series).toEqual(series);
	});

	it.each([
		['without series_id', 'http://localhost/cruises/new'],
		['with a malformed series_id', 'http://localhost/cruises/new?series_id=abc']
	])('throws a 404 %s', async (_, href) => {
		await expect(runLoad(href)).rejects.toMatchObject({
			status: 404,
			body: { title: 'Nie znaleziono serii', backHref: '/series' }
		});
		expect(mockGetSeries).not.toHaveBeenCalled();
	});

	it('throws a 404 for an unknown series', async () => {
		mockGetSeries.mockRejectedValue(new ApiError(404, 'series not found'));
		await expect(runLoad('http://localhost/cruises/new?series_id=99')).rejects.toMatchObject({
			status: 404,
			body: { title: 'Nie znaleziono serii' }
		});
	});
});

describe('new edition page', () => {
	it('links back to its series above the title', async () => {
		await renderPage();
		expect(screen.getByRole('link', { name: 'May I Sail' })).toHaveAttribute('href', '/series/3');
	});

	it('shows the series read-only under the "Nowa edycja" title', async () => {
		await renderPage();
		expect(screen.getByRole('heading', { name: 'Nowa edycja' })).toBeInTheDocument();
		expect(screen.getByLabelText('Seria')).toHaveValue('May I Sail');
		expect(screen.getByLabelText('Seria')).toHaveAttribute('readonly');
		expect(screen.getByLabelText('Edycja *')).toBeRequired();
		expect(screen.getByRole('link', { name: 'Anuluj' })).toHaveAttribute('href', '/series/3');
	});

	it('creates the edition in the series and goes back to the series page', async () => {
		await renderPage();
		await fillRequired('MIS 2026', ' 2026 ');
		await fireEvent.click(screen.getByRole('button', { name: 'Utwórz edycję' }));

		await waitFor(() => expect(goto).toHaveBeenCalledWith('/series/3'));
		expect(mockCreate).toHaveBeenCalledWith(
			expect.objectContaining({ name: 'MIS 2026', series_id: 3, edition: ' 2026 ' })
		);
	});

	it('asks only for edition-wide fields: no kind, dates, ports, crew limit, prices or chat', async () => {
		await renderPage();
		for (const label of [
			'Rodzaj rejsu',
			'Data zaokrętowania',
			'Data wyokrętowania',
			'Port wyjścia',
			'Port docelowy',
			'Maks. liczba uczestników',
			'Waluta',
			'Link do czatu grupy'
		]) {
			expect(screen.queryByLabelText(label)).not.toBeInTheDocument();
		}
		expect(screen.getByLabelText('Kraje')).toBeInTheDocument();
		expect(screen.getByText('Co obejmuje cena etapu')).toBeInTheDocument();
	});

	it('refuses a blank edition label', async () => {
		await renderPage();
		await fillRequired('MIS', '   ');
		await fireEvent.submit(screen.getByRole('button', { name: 'Utwórz edycję' }).closest('form')!);

		expect(await screen.findByText('Podaj oznaczenie edycji')).toBeInTheDocument();
		expect(mockCreate).not.toHaveBeenCalled();
	});

	it('shows the API error and stays on the form', async () => {
		mockCreate.mockRejectedValue(new Error('edition is required'));
		await renderPage();
		await fillRequired('MIS', '2026');
		await fireEvent.click(screen.getByRole('button', { name: 'Utwórz edycję' }));

		expect(await screen.findByText('edition is required')).toBeInTheDocument();
		expect(goto).not.toHaveBeenCalled();
	});
});

describe('new edition page logistics fields', () => {
	it('sends travel notes, minimum age and what the leg price covers', async () => {
		await renderPage();
		await fillRequired('Sycylia', '2026');
		await fireEvent.input(screen.getByLabelText('W cenie (jedna pozycja w wierszu)'), {
			target: { value: 'jacht\npaliwo' }
		});
		await fireEvent.input(screen.getByLabelText('Poza ceną (jedna pozycja w wierszu)'), {
			target: { value: 'dojazd' }
		});
		await fireEvent.input(screen.getByLabelText('Dojazd'), {
			target: { value: 'Tanie loty do Katanii' }
		});
		await fireEvent.input(screen.getByLabelText('Minimalny wiek'), { target: { value: '18' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Utwórz edycję' }));

		expect(mockCreate).toHaveBeenCalledWith({
			name: 'Sycylia',
			series_id: 3,
			edition: '2026',
			countries: '',
			description: '',
			travel_notes: 'Tanie loty do Katanii',
			min_age: 18,
			included: ['jacht', 'paliwo'],
			not_included: ['dojazd']
		});
	});

	it('omits min_age when the field is left empty', async () => {
		await renderPage();
		await fillRequired('Solo', '2026');
		await fireEvent.input(screen.getByLabelText('Minimalny wiek'), { target: { value: '18' } });
		await fireEvent.input(screen.getByLabelText('Minimalny wiek'), { target: { value: '' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Utwórz edycję' }));

		expect(mockCreate).toHaveBeenCalledWith(expect.objectContaining({ min_age: undefined }));
	});
});
