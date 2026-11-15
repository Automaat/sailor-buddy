import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import { makeSeries } from '#lib/test-utils.js';

vi.mock('#lib/api/routes.js', () => ({ listSeries: vi.fn(), createSeries: vi.fn() }));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));

import { listSeries, createSeries } from '#lib/api/routes.js';
import { auth } from '#lib/stores/auth.svelte.js';
import SeriesPage from './+page.svelte';

const mockList = listSeries as unknown as ReturnType<typeof vi.fn>;
const mockCreate = createSeries as unknown as ReturnType<typeof vi.fn>;
const mockAuth = auth as unknown as { isAdmin: boolean };

beforeEach(() => {
	mockList.mockReset().mockResolvedValue([]);
	mockCreate.mockReset();
	vi.spyOn(console, 'error').mockImplementation(() => {});
	mockAuth.isAdmin = true;
});

describe('series page', () => {
	it('shows the empty state when there are no series', async () => {
		render(SeriesPage);
		expect(await screen.findByText('Brak serii')).toBeInTheDocument();
	});

	it('links each series to its page', async () => {
		mockList.mockResolvedValue([
			makeSeries({ id: 4, name: 'AGH Winter Sail Expedition', description: 'Od 2012' })
		]);
		render(SeriesPage);
		const link = (await screen.findByText('AGH Winter Sail Expedition')).closest('a');
		expect(link).toHaveAttribute('href', '/series/4');
		expect(screen.getByText('Od 2012')).toBeInTheDocument();
	});

	it("shows each series' kind on its card", async () => {
		mockList.mockResolvedValue([
			makeSeries({ id: 4, name: 'Mazury', kind: 'inland' }),
			makeSeries({ id: 5, name: 'Bałtyk', kind: 'sea' })
		]);
		render(SeriesPage);
		expect((await screen.findByText('Mazury')).closest('a')).toHaveTextContent('Śródlądowy');
		expect(screen.getByText('Bałtyk').closest('a')).toHaveTextContent('Morski');
	});

	it('lets an admin create a series', async () => {
		mockCreate.mockResolvedValue(makeSeries({ id: 7, name: 'Navigare Necesse Est' }));
		render(SeriesPage);
		await screen.findByText('Brak serii');

		await fireEvent.click(screen.getByRole('button', { name: '+ Dodaj serię' }));
		await fireEvent.input(screen.getByLabelText('Nazwa *'), {
			target: { value: 'Navigare Necesse Est' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj' }));

		expect(mockCreate).toHaveBeenCalledWith({
			name: 'Navigare Necesse Est',
			description: undefined,
			kind: 'sea'
		});
		expect(await screen.findByText('Navigare Necesse Est')).toBeInTheDocument();
	});

	it('creates a series of the picked kind', async () => {
		mockCreate.mockResolvedValue(makeSeries({ id: 8, name: 'Mazury', kind: 'inland' }));
		render(SeriesPage);
		await screen.findByText('Brak serii');

		await fireEvent.click(screen.getByRole('button', { name: '+ Dodaj serię' }));
		expect(screen.getByLabelText('Rodzaj rejsów')).toHaveValue('sea');
		await fireEvent.input(screen.getByLabelText('Nazwa *'), { target: { value: 'Mazury' } });
		await fireEvent.change(screen.getByLabelText('Rodzaj rejsów'), {
			target: { value: 'inland' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj' }));

		expect(mockCreate).toHaveBeenCalledWith({
			name: 'Mazury',
			description: undefined,
			kind: 'inland'
		});
		expect((await screen.findByText('Mazury')).closest('a')).toHaveTextContent('Śródlądowy');
	});

	it('hides the create button for a member', async () => {
		mockAuth.isAdmin = false;
		render(SeriesPage);
		await screen.findByText('Brak serii');
		expect(screen.queryByRole('button', { name: '+ Dodaj serię' })).not.toBeInTheDocument();
	});

	it('shows the error and keeps the form when creating fails', async () => {
		mockCreate.mockRejectedValue(new Error('boom'));
		render(SeriesPage);
		await screen.findByText('Brak serii');
		await fireEvent.click(screen.getByRole('button', { name: '+ Dodaj serię' }));
		await fireEvent.input(screen.getByLabelText('Nazwa *'), { target: { value: 'X' } });
		await fireEvent.input(screen.getByLabelText('Opis'), { target: { value: 'Opis serii' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj' }));

		expect(mockCreate).toHaveBeenCalledWith({ name: 'X', description: 'Opis serii', kind: 'sea' });
		expect(await screen.findByText('boom')).toBeInTheDocument();
	});

	it('shows the empty state when loading fails', async () => {
		mockList.mockRejectedValue(new Error('down'));
		render(SeriesPage);
		expect(await screen.findByText('Brak serii')).toBeInTheDocument();
	});
});
