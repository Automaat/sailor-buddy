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
	createYacht: vi.fn(),
	listCruises: vi.fn(),
	listMembers: vi.fn(), listCrew: vi.fn()
}));

import { getTrip, updateTrip, listYachts, createYacht, listCruises, listMembers, listCrew } from '#lib/api/routes.js';
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
	fn(listCrew).mockResolvedValue([]);
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
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(listCruises).mockRejectedValue(new Error('down'));
		render(EditTripPage);
		const select = await screen.findByLabelText('Wydarzenie klubu *');
		expect(screen.getByRole('option', { name: 'Wydarzenie #13' })).toBeInTheDocument();
		expect(select).toHaveValue('13');
		expect(screen.getByText(/Nie udało się wczytać: lista wydarzeń\./)).toBeInTheDocument();

		expect(await save()).toMatchObject({ cruise_id: 13 });
	});

	it('saves the leg with the yacht still being added', async () => {
		let resolveYacht!: (y: unknown) => void;
		fn(createYacht).mockReturnValue(new Promise((r) => (resolveYacht = r)));
		render(EditTripPage);

		await fireEvent.click(await screen.findByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: 'Selma' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz zmiany' }));
		expect(updateTrip).not.toHaveBeenCalled();

		resolveYacht({ id: 21, name: 'Selma', created_at: '', updated_at: '' });
		await waitFor(() => expect(updateTrip).toHaveBeenCalledTimes(1));
		expect(fn(updateTrip).mock.calls[0][1]).toMatchObject({ yacht_id: 21 });
	});

	it('refuses to save without a cruise', async () => {
		fn(getTrip).mockResolvedValue(makeTrip({ id: 5, cruise_id: 0 }));
		fn(listCruises).mockResolvedValue([]);
		render(EditTripPage);
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));

		expect(await screen.findByText('Wybierz wydarzenie, do którego należy rejs')).toBeInTheDocument();
		expect(updateTrip).not.toHaveBeenCalled();
	});

	it('shows the server error when saving fails', async () => {
		fn(updateTrip).mockRejectedValue(new Error('cruise not found'));
		render(EditTripPage);
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));

		expect(await screen.findByText('cruise not found')).toBeInTheDocument();
	});
});

describe('trip edit page — sign-up deadline', () => {
	it('keeps the saved deadline on a no-change save', async () => {
		fn(getTrip).mockResolvedValue(makeTrip({ id: 5, cruise_id: 13, name: 'Etap 1', embark_date: '2026-07-01', sign_up_deadline: '2026-06-01' }));
		render(EditTripPage);
		expect(await screen.findByLabelText('Zapisy do (włącznie)')).toHaveValue('2026-06-01');
		expect((await save()).sign_up_deadline).toBe('2026-06-01');
	});

	it('sets a deadline no later than the embark date', async () => {
		fn(getTrip).mockResolvedValue(makeTrip({ id: 5, cruise_id: 13, name: 'Etap 1', embark_date: '2026-07-01' }));
		render(EditTripPage);
		const input = await screen.findByLabelText('Zapisy do (włącznie)');
		expect(input).toHaveAttribute('max', '2026-07-01');
		await fireEvent.input(input, { target: { value: '2026-06-15' } });
		expect((await save()).sign_up_deadline).toBe('2026-06-15');
	});

	it('clears the deadline by leaving the field empty', async () => {
		fn(getTrip).mockResolvedValue(makeTrip({ id: 5, cruise_id: 13, name: 'Etap 1', embark_date: '2026-07-01', sign_up_deadline: '2026-06-01' }));
		render(EditTripPage);
		await fireEvent.input(await screen.findByLabelText('Zapisy do (włącznie)'), { target: { value: '' } });
		const body = await save();
		expect(body.sign_up_deadline).toBeUndefined();
	});
});
