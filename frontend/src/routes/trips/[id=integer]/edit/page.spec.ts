import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { makeCruise, makeTrip } from '#lib/test-utils.js';
import { ApiError } from '#lib/api/client.js';

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
import { load } from './+page.js';
import type { PageData } from './$types';

function runLoad(id = '5') {
	return load({ params: { id } } as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

/** renderPage runs the route's load function and renders the page with its data, the way SvelteKit does on navigation. */
async function renderPage(id = '5') {
	return render(EditTripPage, { props: { data: await runLoad(id) } });
}

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
		await renderPage();
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
		await renderPage();
		const select = await screen.findByLabelText('Wydarzenie klubu *');
		await fireEvent.change(select, { target: { value: '14' } });

		expect(await save()).toMatchObject({ cruise_id: 14 });
	});

	it('keeps the link when the cruise list fails to load', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(listCruises).mockRejectedValue(new Error('down'));
		await renderPage();
		const select = await screen.findByLabelText('Wydarzenie klubu *');
		expect(screen.getByRole('option', { name: 'Wydarzenie #13' })).toBeInTheDocument();
		expect(select).toHaveValue('13');
		expect(screen.getByText(/Nie udało się wczytać: lista wydarzeń\./)).toBeInTheDocument();

		expect(await save()).toMatchObject({ cruise_id: 13 });
	});

	it('saves the leg with the yacht still being added', async () => {
		let resolveYacht!: (y: unknown) => void;
		fn(createYacht).mockReturnValue(new Promise((r) => (resolveYacht = r)));
		await renderPage();

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
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));

		expect(await screen.findByText('Wybierz wydarzenie, do którego należy rejs')).toBeInTheDocument();
		expect(updateTrip).not.toHaveBeenCalled();
	});

	it('shows the server error when saving fails', async () => {
		fn(updateTrip).mockRejectedValue(new Error('cruise not found'));
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));

		expect(await screen.findByText('cruise not found')).toBeInTheDocument();
	});
});

describe('trip edit page — sign-up deadline', () => {
	it('keeps the saved deadline on a no-change save', async () => {
		fn(getTrip).mockResolvedValue(makeTrip({ id: 5, cruise_id: 13, name: 'Etap 1', embark_date: '2026-07-01', sign_up_deadline: '2026-06-01' }));
		await renderPage();
		expect(await screen.findByLabelText('Zapisy do (włącznie)')).toHaveValue('2026-06-01');
		expect((await save()).sign_up_deadline).toBe('2026-06-01');
	});

	it('sets a deadline no later than the embark date', async () => {
		fn(getTrip).mockResolvedValue(makeTrip({ id: 5, cruise_id: 13, name: 'Etap 1', embark_date: '2026-07-01' }));
		await renderPage();
		const input = await screen.findByLabelText('Zapisy do (włącznie)');
		expect(input).toHaveAttribute('max', '2026-07-01');
		await fireEvent.input(input, { target: { value: '2026-06-15' } });
		expect((await save()).sign_up_deadline).toBe('2026-06-15');
	});

	it('clears the deadline by leaving the field empty', async () => {
		fn(getTrip).mockResolvedValue(makeTrip({ id: 5, cruise_id: 13, name: 'Etap 1', embark_date: '2026-07-01', sign_up_deadline: '2026-06-01' }));
		await renderPage();
		await fireEvent.input(await screen.findByLabelText('Zapisy do (włącznie)'), { target: { value: '' } });
		const body = await save();
		expect(body.sign_up_deadline).toBeUndefined();
	});
});

describe('trip edit page load', () => {
	it('loads the trip named by the URL', async () => {
		fn(getTrip).mockResolvedValue(makeTrip({ id: 9, cruise_id: 13 }));
		const data = await runLoad('9');

		expect(getTrip).toHaveBeenCalledWith(9);
		expect(data.trip.id).toBe(9);
		expect(data.loadErrors).toEqual([]);
	});

	it('throws a 404 with the trip copy for +error.svelte', async () => {
		fn(getTrip).mockRejectedValue(new ApiError(404, 'not found'));

		await expect(runLoad()).rejects.toMatchObject({
			status: 404,
			body: {
				message: 'Ten rejs nie istnieje lub nie masz do niego dostępu.',
				title: 'Nie znaleziono rejsu',
				backHref: '/cruises'
			}
		});
	});

	it('passes other trip failures on to +error.svelte', async () => {
		const boom = new Error('boom');
		fn(getTrip).mockRejectedValue(boom);

		await expect(runLoad()).rejects.toBe(boom);
	});

	it('fails the load when the yacht list fails, so a save cannot clear the yacht', async () => {
		const boom = new Error('yachts down');
		fn(listYachts).mockRejectedValue(boom);

		await expect(runLoad()).rejects.toBe(boom);
	});

	it('names a failed event list instead of showing no events', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(listCruises).mockRejectedValue(new Error('down'));

		const data = await runLoad();

		expect(data.cruises).toEqual([]);
		expect(data.loadErrors).toEqual(['lista wydarzeń']);
	});
});

describe('trip edit page same-route navigation', () => {
	beforeEach(() => {
		fn(getTrip).mockImplementation(async (id: number) =>
			makeTrip({ id, cruise_id: id === 5 ? 13 : 14, name: id === 5 ? 'Etap 1' : 'Etap 2' })
		);
	});

	it('shows the second trip and drops edits made to the first', async () => {
		const view = await renderPage();
		const name = await screen.findByLabelText('Nazwa rejsu *');
		await fireEvent.input(name, { target: { value: 'Zmieniony etap' } });

		await view.rerender({ data: await runLoad('6') });

		expect(screen.getByLabelText('Nazwa rejsu *')).toHaveValue('Etap 2');
		expect(screen.getByLabelText('Wydarzenie klubu *')).toHaveValue('14');
		expect(screen.getByRole('link', { name: 'Anuluj' })).toHaveAttribute('href', '/trips/6');
		expect(await save()).toMatchObject({ name: 'Etap 2', cruise_id: 14 });
		expect(fn(updateTrip).mock.calls[0][0]).toBe(6);
	});

	it('clears the first trip\'s save error', async () => {
		fn(updateTrip).mockRejectedValueOnce(new Error('cruise not found'));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));
		expect(await screen.findByText('cruise not found')).toBeInTheDocument();

		await view.rerender({ data: await runLoad('6') });

		expect(screen.queryByText('cruise not found')).not.toBeInTheDocument();
	});

	it('drops a save reply for the trip the page has left', async () => {
		let answer: (v?: unknown) => void = () => {};
		fn(updateTrip).mockReturnValueOnce(new Promise((resolve) => (answer = resolve)));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));
		expect(screen.getByRole('button', { name: 'Zapisywanie...' })).toBeDisabled();

		await view.rerender({ data: await runLoad('6') });

		expect(screen.getByRole('button', { name: 'Zapisz zmiany' })).toBeEnabled();
		answer();
		await new Promise((r) => setTimeout(r, 0));
		expect(goto).not.toHaveBeenCalled();
	});

	it('drops a save error for the trip the page has left', async () => {
		let fail: (e: unknown) => void = () => {};
		fn(updateTrip).mockReturnValueOnce(new Promise((_, reject) => (fail = reject)));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));

		await view.rerender({ data: await runLoad('6') });
		fail(new Error('cruise not found'));
		await new Promise((r) => setTimeout(r, 0));

		expect(screen.queryByText('cruise not found')).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Zapisz zmiany' })).toBeEnabled();
	});

	it('does not pick a yacht still being added for the first trip', async () => {
		let resolveYacht!: (y: unknown) => void;
		fn(createYacht).mockReturnValue(new Promise((r) => (resolveYacht = r)));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: 'Selma' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));

		await view.rerender({ data: await runLoad('6') });
		resolveYacht({ id: 21, name: 'Selma', created_at: '', updated_at: '' });
		await new Promise((r) => setTimeout(r, 0));

		expect(screen.getByLabelText('Jacht')).toHaveValue('0');
		expect(updateTrip).not.toHaveBeenCalled();
	});
});
