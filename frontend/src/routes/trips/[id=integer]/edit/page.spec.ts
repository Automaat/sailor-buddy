import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import type { Cruise } from '#lib/api/types.js';
import { makeTrip } from '#lib/test-utils.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: { params: { id: '7' } } }));
vi.mock('#lib/api/routes.js', () => ({
	getTrip: vi.fn(),
	updateTrip: vi.fn(),
	listYachts: vi.fn(),
	listCruises: vi.fn(),
	listMembers: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));

import { goto } from '$app/navigation';
import { getTrip, updateTrip, listYachts, listCruises, listMembers } from '#lib/api/routes.js';
import EditTripPage from './+page.svelte';

const mockGetTrip = getTrip as unknown as ReturnType<typeof vi.fn>;
const mockUpdate = updateTrip as unknown as ReturnType<typeof vi.fn>;
const mockListCruises = listCruises as unknown as ReturnType<typeof vi.fn>;

function makeCruise(id: number, name: string): Cruise {
	return {
		id,
		name,
		kind: 'sea',
		currency: 'PLN',
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z'
	} as Cruise;
}

beforeEach(() => {
	mockGetTrip.mockReset().mockResolvedValue(
		makeTrip({
			id: 7,
			name: 'Etap 1',
			cruise_id: 13,
			image_photo_url: '/uploads/photo.jpg'
		})
	);
	mockUpdate.mockReset().mockResolvedValue(undefined);
	mockListCruises.mockReset().mockResolvedValue([makeCruise(13, 'Sycylia 2026'), makeCruise(14, 'Chorwacja 2026')]);
	vi.mocked(listYachts).mockReset().mockResolvedValue([]);
	vi.mocked(listMembers).mockReset().mockResolvedValue([]);
	vi.mocked(goto).mockReset();
});

async function save() {
	await fireEvent.click(screen.getByRole('button', { name: 'Zapisz zmiany' }));
	await waitFor(() => expect(mockUpdate).toHaveBeenCalled());
	return mockUpdate.mock.calls[0];
}

describe('trip edit page', () => {
	it('keeps the cruise link and images on a no-change save', async () => {
		render(EditTripPage);
		await screen.findByRole('option', { name: 'Sycylia 2026' });

		const [id, body] = await save();

		expect(id).toBe(7);
		expect(body).toEqual(
			expect.objectContaining({ name: 'Etap 1', cruise_id: 13, image_photo_url: '/uploads/photo.jpg' })
		);
		expect(goto).toHaveBeenCalledWith('/trips/7');
	});

	it('moves the trip to another cruise', async () => {
		render(EditTripPage);
		await screen.findByRole('option', { name: 'Chorwacja 2026' });

		await fireEvent.change(screen.getByLabelText('Wydarzenie klubu'), { target: { value: '14' } });
		const [, body] = await save();

		expect(body).toEqual(expect.objectContaining({ cruise_id: 14 }));
	});

	it('detaches the trip from its cruise', async () => {
		render(EditTripPage);
		await screen.findByRole('option', { name: 'Sycylia 2026' });

		await fireEvent.change(screen.getByLabelText('Wydarzenie klubu'), { target: { value: '0' } });
		const [, body] = await save();

		expect(body.cruise_id).toBeUndefined();
	});

	it('keeps the cruise link when the cruise list fails to load', async () => {
		mockListCruises.mockRejectedValue(new Error('boom'));
		render(EditTripPage);
		await screen.findByRole('option', { name: 'Wydarzenie #13' });

		const [, body] = await save();

		expect(body).toEqual(expect.objectContaining({ cruise_id: 13 }));
	});

	it('sends no cruise for a standalone trip', async () => {
		mockGetTrip.mockResolvedValue(makeTrip({ id: 7, name: 'Solo' }));
		render(EditTripPage);
		await screen.findByRole('option', { name: 'Sycylia 2026' });

		const [, body] = await save();

		expect(body.cruise_id).toBeUndefined();
	});
});
