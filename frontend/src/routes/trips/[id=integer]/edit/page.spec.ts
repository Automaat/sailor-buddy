import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { makeCruise, makeTrip } from '#lib/test-utils.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: { params: { id: '5' }, url: new URL('http://localhost/trips/5/edit') } }));
vi.mock('#lib/stores/auth.svelte.js', () => ({ auth: { isAdmin: true, user: { id: 1, role: 'admin' } } }));
vi.mock('#lib/api/routes.js', () => ({
	getTrip: vi.fn(),
	updateTrip: vi.fn(),
	listYachts: vi.fn(),
	listCruises: vi.fn(),
	listMembers: vi.fn()
}));

import { getTrip, updateTrip, listYachts, listCruises, listMembers } from '#lib/api/routes.js';
import { goto } from '$app/navigation';
import EditTripPage from './+page.svelte';

const fn = (f: unknown) => f as ReturnType<typeof vi.fn>;

beforeEach(() => {
	vi.clearAllMocks();
	fn(getTrip).mockResolvedValue(
		makeTrip({
			id: 5,
			cruise_id: 13,
			name: 'Etap 1',
			image_logo_url: '/uploads/logo.png',
			image_photo_url: '/uploads/photo.png',
			image_route_url: '/uploads/route.png'
		})
	);
	fn(listYachts).mockResolvedValue([]);
	fn(listMembers).mockResolvedValue([]);
	fn(listCruises).mockResolvedValue([makeCruise({ id: 13, name: 'NNE 2025' }), makeCruise({ id: 14, name: 'WRM 2025' })]);
	fn(updateTrip).mockResolvedValue(undefined);
});

async function save() {
	await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));
	await waitFor(() => expect(updateTrip).toHaveBeenCalled());
	return fn(updateTrip).mock.calls[0][1];
}

describe('trip edit page', () => {
	it('keeps the cruise link and images on a no-change save', async () => {
		render(EditTripPage);
		expect(await screen.findByLabelText('Wydarzenie klubu *')).toHaveValue('13');

		const body = await save();

		expect(body).toMatchObject({
			cruise_id: 13,
			image_logo_url: '/uploads/logo.png',
			image_photo_url: '/uploads/photo.png',
			image_route_url: '/uploads/route.png'
		});
		expect(goto).toHaveBeenCalledWith('/trips/5');
	});

	it('moves the trip to another cruise', async () => {
		render(EditTripPage);
		const select = await screen.findByLabelText('Wydarzenie klubu *');
		await fireEvent.change(select, { target: { value: '14' } });

		expect(await save()).toMatchObject({ cruise_id: 14 });
	});

	it('keeps the link when the cruise list fails to load', async () => {
		fn(listCruises).mockRejectedValue(new Error('down'));
		render(EditTripPage);
		const select = await screen.findByLabelText('Wydarzenie klubu *');
		expect(screen.getByRole('option', { name: 'Wydarzenie #13' })).toBeInTheDocument();
		expect(select).toHaveValue('13');

		expect(await save()).toMatchObject({ cruise_id: 13 });
	});
});
