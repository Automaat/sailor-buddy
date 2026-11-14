import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/api/routes.js', () => ({ createCruise: vi.fn(), listSeries: vi.fn() }));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));

import { goto } from '$app/navigation';
import { createCruise, listSeries } from '#lib/api/routes.js';
import NewCruisePage from './+page.svelte';

const mockCreate = createCruise as unknown as ReturnType<typeof vi.fn>;
const mockListSeries = listSeries as unknown as ReturnType<typeof vi.fn>;

beforeEach(() => {
	mockCreate.mockReset().mockResolvedValue({ id: 5 });
	mockListSeries.mockReset().mockResolvedValue([
		{
			id: 3,
			name: 'May I Sail',
			created_at: '2026-01-01T00:00:00Z',
			updated_at: '2026-01-01T00:00:00Z'
		}
	]);
	vi.mocked(goto).mockReset();
});

describe('new cruise page series fields', () => {
	it('sends the chosen series and edition', async () => {
		render(NewCruisePage);
		await screen.findByRole('option', { name: 'May I Sail' });

		await fireEvent.input(screen.getByLabelText('Nazwa wydarzenia *'), {
			target: { value: 'MIS 2026' }
		});
		await fireEvent.change(screen.getByLabelText('Seria'), { target: { value: '3' } });
		await fireEvent.input(screen.getByLabelText('Edycja'), { target: { value: '2026' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Utwórz wydarzenie' }));

		expect(mockCreate).toHaveBeenCalledWith(
			expect.objectContaining({ name: 'MIS 2026', series_id: 3, edition: '2026' })
		);
		expect(goto).toHaveBeenCalledWith('/cruises/5');
	});

	it('omits series_id when no series is chosen', async () => {
		render(NewCruisePage);
		await screen.findByRole('option', { name: 'May I Sail' });

		await fireEvent.input(screen.getByLabelText('Nazwa wydarzenia *'), {
			target: { value: 'Solo' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Utwórz wydarzenie' }));

		expect(mockCreate).toHaveBeenCalledWith(
			expect.objectContaining({ name: 'Solo', series_id: undefined, edition: '' })
		);
	});
});

describe('new cruise page logistics fields', () => {
	it('sends the group chat link, travel notes and minimum age', async () => {
		render(NewCruisePage);
		await screen.findByRole('option', { name: 'May I Sail' });

		await fireEvent.input(screen.getByLabelText('Nazwa wydarzenia *'), {
			target: { value: 'Sycylia' }
		});
		await fireEvent.input(screen.getByLabelText('Link do czatu grupy'), {
			target: { value: 'https://discord.gg/sycylia' }
		});
		await fireEvent.input(screen.getByLabelText('Dojazd'), {
			target: { value: 'Tanie loty do Katanii' }
		});
		await fireEvent.input(screen.getByLabelText('Minimalny wiek'), { target: { value: '18' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Utwórz wydarzenie' }));

		expect(mockCreate).toHaveBeenCalledWith(
			expect.objectContaining({
				group_chat_url: 'https://discord.gg/sycylia',
				travel_notes: 'Tanie loty do Katanii',
				min_age: 18
			})
		);
	});

	it('omits min_age when the field is left empty', async () => {
		render(NewCruisePage);
		await screen.findByRole('option', { name: 'May I Sail' });

		await fireEvent.input(screen.getByLabelText('Nazwa wydarzenia *'), {
			target: { value: 'Solo' }
		});
		await fireEvent.input(screen.getByLabelText('Minimalny wiek'), { target: { value: '18' } });
		await fireEvent.input(screen.getByLabelText('Minimalny wiek'), { target: { value: '' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Utwórz wydarzenie' }));

		expect(mockCreate).toHaveBeenCalledWith(expect.objectContaining({ min_age: undefined }));
	});
});
