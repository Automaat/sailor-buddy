import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { makeCruise, makeVoyage } from '#lib/test-utils.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: { params: { id: '8' }, url: new URL('http://localhost/voyages/8/edit') } }));
vi.mock('#lib/stores/auth.svelte.js', () => ({ auth: { isAdmin: true, user: { id: 1, role: 'admin' } } }));
vi.mock('#lib/api/routes.js', () => ({
	getVoyage: vi.fn(),
	updateVoyage: vi.fn(),
	listYachts: vi.fn(),
	createYacht: vi.fn(),
	listCruises: vi.fn(),
	listMembers: vi.fn(), listCrew: vi.fn()
}));

import { getVoyage, updateVoyage, listYachts, listCruises, listMembers, listCrew } from '#lib/api/routes.js';
import EditVoyagePage from './+page.svelte';

const fn = (f: unknown) => f as ReturnType<typeof vi.fn>;

beforeEach(() => {
	vi.clearAllMocks();
	fn(getVoyage).mockResolvedValue(
		makeVoyage({ id: 8, cruise_id: 13, name: 'Etap 1', image_photo_url: '/uploads/photo.png', image_route_url: '/uploads/route.png' })
	);
	fn(listYachts).mockResolvedValue([]);
	fn(listMembers).mockResolvedValue([]);
	fn(listCrew).mockResolvedValue([]);
	fn(listCruises).mockResolvedValue([makeCruise({ id: 13, name: 'NNE 2025' })]);
	fn(updateVoyage).mockResolvedValue(undefined);
});

describe('voyage edit page', () => {
	it('keeps the cruise link and images on a no-change save', async () => {
		render(EditVoyagePage);
		expect(await screen.findByLabelText('Wydarzenie klubu *')).toHaveValue('13');

		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz zmiany' }));

		await waitFor(() => expect(updateVoyage).toHaveBeenCalled());
		expect(fn(updateVoyage).mock.calls[0][1]).toMatchObject({
			cruise_id: 13,
			image_photo_url: '/uploads/photo.png',
			image_route_url: '/uploads/route.png'
		});
	});

	it('refuses to save without a cruise', async () => {
		fn(getVoyage).mockResolvedValue(makeVoyage({ id: 5, cruise_id: 0 }));
		fn(listCruises).mockResolvedValue([]);
		render(EditVoyagePage);
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));

		expect(await screen.findByText('Wybierz wydarzenie, do którego należy rejs')).toBeInTheDocument();
		expect(updateVoyage).not.toHaveBeenCalled();
	});

	it('shows the server error when saving fails', async () => {
		fn(updateVoyage).mockRejectedValue(new Error('cruise not found'));
		render(EditVoyagePage);
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));

		expect(await screen.findByText('cruise not found')).toBeInTheDocument();
	});
});
