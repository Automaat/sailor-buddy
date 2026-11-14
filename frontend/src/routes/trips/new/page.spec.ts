import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { makeCruise, makePerson, makeTrip } from '#lib/test-utils.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
const authState = vi.hoisted(() => ({
	isAdmin: true,
	user: { id: 1, role: 'admin', is_captain: false, person_id: undefined as number | undefined }
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({ auth: authState }));
vi.mock('#lib/api/routes.js', () => ({ listCruises: vi.fn(), createTrip: vi.fn(), listMembers: vi.fn(), listCrew: vi.fn() }));

import { listCruises, createTrip, listMembers, listCrew } from '#lib/api/routes.js';
import { goto } from '$app/navigation';
import NewTripPage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

function runLoad(href = 'http://localhost/trips/new') {
	return load({ url: new URL(href) } as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

/** renderPage runs the route's load function and renders the page with its data, the way SvelteKit does on navigation. */
async function renderPage(href?: string) {
	return render(NewTripPage, { props: { data: await runLoad(href) } });
}

const fn = (f: unknown) => f as ReturnType<typeof vi.fn>;

beforeEach(() => {
	vi.clearAllMocks();
	authState.user = { id: 1, role: 'admin', is_captain: false, person_id: undefined };
	fn(listMembers).mockResolvedValue([]);
	fn(listCrew).mockResolvedValue([]);
	fn(listCruises).mockResolvedValue([makeCruise({ id: 13, name: 'NNE 2025', start_port: 'Turku' })]);
	fn(createTrip).mockResolvedValue(makeTrip({ id: 77 }));
});

describe('new trip page', () => {
	it('refuses to submit without a cruise', async () => {
		await renderPage();
		await fireEvent.input(await screen.findByLabelText('Nazwa rejsu *'), { target: { value: 'Etap 1' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Zaplanuj rejs' }));

		expect(await screen.findByText('Wybierz wydarzenie, do którego należy rejs')).toBeInTheDocument();
		expect(createTrip).not.toHaveBeenCalled();
	});

	it('preselects and prefills the cruise from the URL and creates the leg in it', async () => {
		await renderPage('http://localhost/trips/new?cruise_id=13');
		await waitFor(() => expect(screen.getByLabelText('Wydarzenie klubu *')).toHaveValue('13'));
		expect(screen.getByLabelText('Port wyjścia')).toHaveValue('Turku');
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), { target: { value: 'Etap 1' } });

		await fireEvent.click(screen.getByRole('button', { name: 'Zaplanuj rejs' }));

		await waitFor(() => expect(createTrip).toHaveBeenCalled());
		expect(fn(createTrip).mock.calls[0][0]).toMatchObject({ cruise_id: 13, name: 'Etap 1' });
		expect(goto).toHaveBeenCalledWith('/trips/77');
	});
});

describe('new trip page captain', () => {
	it("preselects a captain's own person as the leg captain", async () => {
		authState.user = { id: 1, role: 'admin', is_captain: true, person_id: 40 };
		fn(listCrew).mockResolvedValue([makePerson({ id: 40, full_name: 'Kasia Admin', user_id: 1, has_account: true })]);
		await renderPage('http://localhost/trips/new?cruise_id=13');
		await screen.findByRole('option', { name: 'Kasia Admin' });
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), { target: { value: 'Etap 1' } });

		await fireEvent.click(screen.getByRole('button', { name: 'Zaplanuj rejs' }));

		await waitFor(() => expect(createTrip).toHaveBeenCalled());
		expect(fn(createTrip).mock.calls[0][0]).toMatchObject({ captain_person_id: 40 });
	});
});

describe('new trip page event switch', () => {
	it('replaces untouched prefilled fields but keeps edited ones', async () => {
		fn(listCruises).mockResolvedValue([
			makeCruise({ id: 13, name: 'NNE', start_port: 'Turku', end_port: 'Vaasa', cost_per_person: 900 }),
			makeCruise({ id: 14, name: 'WRM', start_port: 'Giżycko', end_port: 'Mikołajki', cost_per_person: 870 })
		]);
		await renderPage('http://localhost/trips/new?cruise_id=13');
		await waitFor(() => expect(screen.getByLabelText('Port wyjścia')).toHaveValue('Turku'));
		await fireEvent.input(screen.getByLabelText('Port docelowy'), { target: { value: 'Sztokholm' } });

		await fireEvent.change(screen.getByLabelText('Wydarzenie klubu *'), { target: { value: '14' } });

		expect(screen.getByLabelText('Port wyjścia')).toHaveValue('Giżycko');
		expect(screen.getByLabelText('Port docelowy')).toHaveValue('Sztokholm');
	});
});

describe('new trip page load errors', () => {
	it('names the failed cruise list and still renders the form', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(listCruises).mockRejectedValue(new Error('down'));
		await renderPage();

		expect(await screen.findByText(/Nie udało się wczytać: lista wydarzeń\./)).toBeInTheDocument();
		expect(screen.getByLabelText('Nazwa rejsu *')).toBeInTheDocument();
	});
});

describe('new trip page load', () => {
	it('reads the preselected event from ?cruise_id=', async () => {
		const data = await runLoad('http://localhost/trips/new?cruise_id=13');

		expect(data.cruiseId).toBe(13);
		expect(data.cruises.map((c) => c.id)).toEqual([13]);
		expect(data.loadErrors).toEqual([]);
	});

	it.each(['', '?cruise_id=abc', '?cruise_id=-3', '?cruise_id=1.5'])('preselects no event for "%s"', async (query) => {
		expect((await runLoad(`http://localhost/trips/new${query}`)).cruiseId).toBe(0);
	});

	it('names a failed event list instead of showing no events', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(listCruises).mockRejectedValue(new Error('down'));

		const data = await runLoad();

		expect(data.cruises).toEqual([]);
		expect(data.loadErrors).toEqual(['lista wydarzeń']);
	});
});

describe('new trip page same-route navigation', () => {
	beforeEach(() => {
		fn(listCruises).mockResolvedValue([
			makeCruise({ id: 13, name: 'NNE', start_port: 'Turku' }),
			makeCruise({ id: 14, name: 'WRM', start_port: 'Giżycko' })
		]);
	});

	it('starts a fresh draft for the second event', async () => {
		const view = await renderPage('http://localhost/trips/new?cruise_id=13');
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), { target: { value: 'Etap 1' } });
		await fireEvent.input(screen.getByLabelText('Port wyjścia'), { target: { value: 'Helsinki' } });

		await view.rerender({ data: await runLoad('http://localhost/trips/new?cruise_id=14') });

		expect(screen.getByLabelText('Wydarzenie klubu *')).toHaveValue('14');
		expect(screen.getByLabelText('Port wyjścia')).toHaveValue('Giżycko');
		expect(screen.getByLabelText('Nazwa rejsu *')).toHaveValue('');
		expect(screen.getByRole('link', { name: 'Anuluj' })).toHaveAttribute('href', '/cruises/14');
	});

	it('clears the first draft\'s error', async () => {
		const view = await renderPage();
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), { target: { value: 'Etap 1' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Zaplanuj rejs' }));
		expect(await screen.findByText('Wybierz wydarzenie, do którego należy rejs')).toBeInTheDocument();

		await view.rerender({ data: await runLoad('http://localhost/trips/new?cruise_id=14') });

		expect(screen.queryByText('Wybierz wydarzenie, do którego należy rejs')).not.toBeInTheDocument();
	});

	it('drops a create reply for the draft the page has left', async () => {
		let answer: (v: unknown) => void = () => {};
		fn(createTrip).mockReturnValueOnce(new Promise((resolve) => (answer = resolve)));
		const view = await renderPage('http://localhost/trips/new?cruise_id=13');
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), { target: { value: 'Etap 1' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Zaplanuj rejs' }));
		expect(screen.getByRole('button', { name: 'Tworzenie...' })).toBeDisabled();

		await view.rerender({ data: await runLoad('http://localhost/trips/new?cruise_id=14') });

		expect(screen.getByRole('button', { name: 'Zaplanuj rejs' })).toBeEnabled();
		answer(makeTrip({ id: 77 }));
		await new Promise((r) => setTimeout(r, 0));
		expect(goto).not.toHaveBeenCalled();
	});

	it("preselects a captain's own person again in the new draft", async () => {
		authState.user = { id: 1, role: 'admin', is_captain: true, person_id: 40 };
		fn(listCrew).mockResolvedValue([makePerson({ id: 40, full_name: 'Kasia Admin', user_id: 1, has_account: true })]);
		const view = await renderPage('http://localhost/trips/new?cruise_id=13');
		await screen.findByRole('option', { name: 'Kasia Admin' });
		await fireEvent.change(screen.getByLabelText('Kapitan'), { target: { value: '0' } });

		await view.rerender({ data: await runLoad('http://localhost/trips/new?cruise_id=14') });

		await waitFor(() => expect(screen.getByLabelText('Kapitan')).toHaveValue('40'));
	});
});
