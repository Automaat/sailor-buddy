import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { makeCruise, makeVoyage } from '#lib/test-utils.js';
import { ApiError } from '#lib/api/client.js';

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

import { getVoyage, updateVoyage, listYachts, createYacht, listCruises, listMembers, listCrew } from '#lib/api/routes.js';
import { goto } from '$app/navigation';
import EditVoyagePage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

function runLoad(id = '8') {
	return load({ params: { id } } as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

/** renderPage runs the route's load function and renders the page with its data, the way SvelteKit does on navigation. */
async function renderPage(id = '8') {
	return render(EditVoyagePage, { props: { data: await runLoad(id) } });
}

async function save() {
	await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));
	await waitFor(() => expect(updateVoyage).toHaveBeenCalled());
	return fn(updateVoyage).mock.calls[0];
}

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
		await renderPage();
		expect(await screen.findByLabelText('Wydarzenie klubu *')).toHaveValue('13');

		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz zmiany' }));

		await waitFor(() => expect(updateVoyage).toHaveBeenCalled());
		expect(fn(updateVoyage).mock.calls[0][1]).toMatchObject({
			cruise_id: 13,
			image_photo_url: '/uploads/photo.png',
			image_route_url: '/uploads/route.png'
		});
	});

	it('saves the voyage with the yacht still being added', async () => {
		let resolveYacht!: (y: unknown) => void;
		fn(createYacht).mockReturnValue(new Promise((r) => (resolveYacht = r)));
		await renderPage();
		expect(await screen.findByLabelText('Wydarzenie klubu *')).toHaveValue('13');

		await fireEvent.click(screen.getByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: 'Selma' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz zmiany' }));
		expect(updateVoyage).not.toHaveBeenCalled();

		resolveYacht({ id: 21, name: 'Selma', created_at: '', updated_at: '' });
		await waitFor(() => expect(updateVoyage).toHaveBeenCalledTimes(1));
		expect(fn(updateVoyage).mock.calls[0][1]).toMatchObject({ yacht_id: 21 });
	});

	it('refuses to save without a cruise', async () => {
		fn(getVoyage).mockResolvedValue(makeVoyage({ id: 5, cruise_id: 0 }));
		fn(listCruises).mockResolvedValue([]);
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));

		expect(await screen.findByText('Wybierz wydarzenie, do którego należy rejs')).toBeInTheDocument();
		expect(updateVoyage).not.toHaveBeenCalled();
	});

	it('shows the server error when saving fails', async () => {
		fn(updateVoyage).mockRejectedValue(new Error('cruise not found'));
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));

		expect(await screen.findByText('cruise not found')).toBeInTheDocument();
	});
});

describe('voyage edit page load errors', () => {
	it('names the failed cruise list and still renders the form', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(listCruises).mockRejectedValue(new Error('down'));
		await renderPage();

		expect(await screen.findByText(/Nie udało się wczytać: lista wydarzeń\./)).toBeInTheDocument();
		expect(screen.getByLabelText('Nazwa rejsu *')).toHaveValue('Etap 1');
	});
});

describe('voyage edit page load', () => {
	it('loads the voyage named by the URL', async () => {
		fn(getVoyage).mockResolvedValue(makeVoyage({ id: 9, cruise_id: 13 }));
		const data = await runLoad('9');

		expect(getVoyage).toHaveBeenCalledWith(9);
		expect(data.voyage.id).toBe(9);
		expect(data.loadErrors).toEqual([]);
	});

	it('throws a 404 with the voyage copy for +error.svelte', async () => {
		fn(getVoyage).mockRejectedValue(new ApiError(404, 'not found'));

		await expect(runLoad()).rejects.toMatchObject({
			status: 404,
			body: {
				message: 'Ten zrealizowany rejs nie istnieje lub nie masz do niego dostępu.',
				title: 'Nie znaleziono rejsu',
				backHref: '/cruises?tab=completed'
			}
		});
	});

	it('passes other voyage failures on to +error.svelte', async () => {
		const boom = new Error('boom');
		fn(getVoyage).mockRejectedValue(boom);

		await expect(runLoad()).rejects.toBe(boom);
	});

	it('fails the load when the yacht list fails, so a save cannot clear the yacht', async () => {
		const boom = new Error('yachts down');
		fn(listYachts).mockRejectedValue(boom);

		await expect(runLoad()).rejects.toBe(boom);
	});
});

describe('voyage edit page same-route navigation', () => {
	beforeEach(() => {
		fn(getVoyage).mockImplementation(async (id: number) =>
			makeVoyage({ id, cruise_id: 13, name: id === 8 ? 'Etap 1' : 'Etap 2', miles: id === 8 ? 120 : 340 })
		);
	});

	it('shows the second voyage and drops edits made to the first', async () => {
		const view = await renderPage();
		await fireEvent.input(await screen.findByLabelText('Nazwa rejsu *'), { target: { value: 'Zmieniony etap' } });

		await view.rerender({ data: await runLoad('9') });

		expect(screen.getByLabelText('Nazwa rejsu *')).toHaveValue('Etap 2');
		expect(screen.getByRole('link', { name: 'Anuluj' })).toHaveAttribute('href', '/voyages/9');
		const [id, body] = await save();
		expect(id).toBe(9);
		expect(body).toMatchObject({ name: 'Etap 2', miles: 340 });
	});

	it('clears the first voyage\'s save error', async () => {
		fn(updateVoyage).mockRejectedValueOnce(new Error('cruise not found'));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));
		expect(await screen.findByText('cruise not found')).toBeInTheDocument();

		await view.rerender({ data: await runLoad('9') });

		expect(screen.queryByText('cruise not found')).not.toBeInTheDocument();
	});

	it('drops a save reply for the voyage the page has left', async () => {
		let answer: (v?: unknown) => void = () => {};
		fn(updateVoyage).mockReturnValueOnce(new Promise((resolve) => (answer = resolve)));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz zmiany' }));
		expect(screen.getByRole('button', { name: 'Zapisywanie...' })).toBeDisabled();

		await view.rerender({ data: await runLoad('9') });

		expect(screen.getByRole('button', { name: 'Zapisz zmiany' })).toBeEnabled();
		answer();
		await new Promise((r) => setTimeout(r, 0));
		expect(goto).not.toHaveBeenCalled();
	});

	it('does not pick a yacht still being added for the first voyage', async () => {
		let resolveYacht!: (y: unknown) => void;
		fn(createYacht).mockReturnValue(new Promise((r) => (resolveYacht = r)));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: 'Selma' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));

		await view.rerender({ data: await runLoad('9') });
		resolveYacht({ id: 21, name: 'Selma', created_at: '', updated_at: '' });
		await new Promise((r) => setTimeout(r, 0));

		expect(screen.getByLabelText('Jacht')).toHaveValue('0');
		expect(updateVoyage).not.toHaveBeenCalled();
	});
});
