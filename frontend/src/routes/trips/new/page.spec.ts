import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { makeCruise, makePerson, makeSeries, makeTrip } from '#lib/test-utils.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
const authState = vi.hoisted(() => ({
	isAdmin: true,
	user: { id: 1, role: 'admin', is_captain: false, person_id: undefined as number | undefined }
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({ auth: authState }));
vi.mock('#lib/api/routes.js', () => ({
	listCruises: vi.fn(),
	listSeries: vi.fn(),
	createTrip: vi.fn(),
	listMembers: vi.fn(),
	listCrew: vi.fn()
}));

import { listCruises, listSeries, createTrip, listMembers, listCrew } from '#lib/api/routes.js';
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
	fn(listCruises).mockResolvedValue([
		makeCruise({ id: 13, name: 'NNE 2025', countries: 'Finlandia', series_id: 2, edition: '2025' }),
		makeCruise({ id: 20, name: 'Rejs na Hel' })
	]);
	fn(listSeries).mockResolvedValue([makeSeries({ id: 2, name: 'NNE' })]);
	fn(createTrip).mockResolvedValue(makeTrip({ id: 77 }));
});

async function nameTrip(name = 'Etap 1') {
	await fireEvent.input(await screen.findByLabelText('Nazwa rejsu *'), { target: { value: name } });
}

function submit() {
	return fireEvent.click(screen.getByRole('button', { name: 'Zaplanuj rejs' }));
}

describe('new trip page', () => {
	it.each([
		['the Rejsy list without a preselected edition', undefined, 'Rejsy', '/series'],
		['the edition its link preselects', '?cruise_id=13', 'NNE 2025', '/cruises/13']
	])('links back to trips above the title', async (_, query, label, href) => {
		await renderPage(query ? `http://localhost/trips/new${query}` : undefined);
		expect(screen.getByTestId('back-link')).toHaveAttribute('href', href);
		expect(screen.getByTestId('back-link')).toHaveAccessibleName(label);
	});

	it('creates a standalone trip by default', async () => {
		await renderPage();
		expect(screen.getByLabelText('Seria')).toHaveValue('standalone');
		expect(screen.queryByLabelText('Edycja *')).not.toBeInTheDocument();
		await nameTrip();

		await submit();

		await waitFor(() => expect(createTrip).toHaveBeenCalled());
		expect(fn(createTrip).mock.calls[0][0]).toMatchObject({ name: 'Etap 1', cruise_id: undefined });
		expect(goto).toHaveBeenCalledWith('/trips/77');
	});

	it('refuses to submit a series without an edition', async () => {
		await renderPage();
		await nameTrip();
		await fireEvent.change(screen.getByLabelText('Seria'), { target: { value: '2' } });

		await submit();

		expect(await screen.findByText('Wybierz edycję, do której należy rejs')).toBeInTheDocument();
		expect(createTrip).not.toHaveBeenCalled();
	});

	it('creates the trip in the edition picked from its series', async () => {
		await renderPage();
		await nameTrip();
		await fireEvent.change(screen.getByLabelText('Seria'), { target: { value: '2' } });
		expect(screen.getByRole('option', { name: '2025 — NNE 2025' })).toBeInTheDocument();
		expect(screen.queryByRole('option', { name: 'Rejs na Hel' })).not.toBeInTheDocument();
		await fireEvent.change(screen.getByLabelText('Edycja *'), { target: { value: '13' } });
		expect(screen.getByLabelText('Kraje')).toHaveValue('Finlandia');

		await submit();

		await waitFor(() => expect(createTrip).toHaveBeenCalled());
		expect(fn(createTrip).mock.calls[0][0]).toMatchObject({ cruise_id: 13, name: 'Etap 1' });
	});

	it('offers cruises without a series under "Bez serii"', async () => {
		await renderPage();
		await nameTrip();
		await fireEvent.change(screen.getByLabelText('Seria'), { target: { value: 'none' } });
		expect(screen.queryByRole('option', { name: '2025 — NNE 2025' })).not.toBeInTheDocument();
		await fireEvent.change(screen.getByLabelText('Edycja *'), { target: { value: '20' } });

		await submit();

		await waitFor(() => expect(createTrip).toHaveBeenCalled());
		expect(fn(createTrip).mock.calls[0][0]).toMatchObject({ cruise_id: 20 });
	});

	it('drops the picked edition when the series changes back to standalone', async () => {
		await renderPage();
		await nameTrip();
		await fireEvent.change(screen.getByLabelText('Seria'), { target: { value: '2' } });
		await fireEvent.change(screen.getByLabelText('Edycja *'), { target: { value: '13' } });
		await fireEvent.change(screen.getByLabelText('Seria'), { target: { value: 'standalone' } });

		await submit();

		await waitFor(() => expect(createTrip).toHaveBeenCalled());
		expect(fn(createTrip).mock.calls[0][0]).toMatchObject({ cruise_id: undefined });
	});

	it('preselects, locks and prefills the edition from the URL and creates the leg in it', async () => {
		await renderPage('http://localhost/trips/new?cruise_id=13');
		await waitFor(() => expect(screen.getByLabelText('Edycja *')).toHaveValue('13'));
		expect(screen.getByLabelText('Seria')).toHaveValue('2');
		expect(screen.getByLabelText('Seria')).toBeDisabled();
		expect(screen.getByLabelText('Edycja *')).toBeDisabled();
		expect(screen.getByLabelText('Kraje')).toHaveValue('Finlandia');
		await nameTrip();

		await submit();

		await waitFor(() => expect(createTrip).toHaveBeenCalled());
		expect(fn(createTrip).mock.calls[0][0]).toMatchObject({ cruise_id: 13, name: 'Etap 1' });
		expect(goto).toHaveBeenCalledWith('/trips/77');
	});
});

describe('new trip page captain', () => {
	it("preselects a captain's own person as the leg captain", async () => {
		authState.user = { id: 1, role: 'admin', is_captain: true, person_id: 40 };
		fn(listCrew).mockResolvedValue([
			makePerson({ id: 40, full_name: 'Kasia Admin', user_id: 1, has_account: true })
		]);
		await renderPage('http://localhost/trips/new?cruise_id=13');
		await screen.findByRole('option', { name: 'Kasia Admin' });
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), { target: { value: 'Etap 1' } });

		await fireEvent.click(screen.getByRole('button', { name: 'Zaplanuj rejs' }));

		await waitFor(() => expect(createTrip).toHaveBeenCalled());
		expect(fn(createTrip).mock.calls[0][0]).toMatchObject({ captain_person_id: 40 });
	});
});

describe('new trip page event switch', () => {
	beforeEach(() => {
		fn(listCruises).mockResolvedValue([
			makeCruise({ id: 13, name: 'NNE', countries: 'Finlandia' }),
			makeCruise({ id: 14, name: 'WRM', countries: 'Polska' })
		]);
	});

	async function pick(id: string) {
		await fireEvent.change(screen.getByLabelText('Edycja *'), { target: { value: id } });
	}

	it("replaces untouched prefilled countries with the next event's", async () => {
		await renderPage();
		await fireEvent.change(await screen.findByLabelText('Seria'), { target: { value: 'none' } });
		await pick('13');
		expect(screen.getByLabelText('Kraje')).toHaveValue('Finlandia');

		await pick('14');

		expect(screen.getByLabelText('Kraje')).toHaveValue('Polska');
	});

	it('keeps countries the admin edited', async () => {
		await renderPage();
		await fireEvent.change(await screen.findByLabelText('Seria'), { target: { value: 'none' } });
		await pick('13');
		await fireEvent.input(screen.getByLabelText('Kraje'), { target: { value: 'Szwecja' } });

		await pick('14');

		expect(screen.getByLabelText('Kraje')).toHaveValue('Szwecja');
	});

	it('copies no dates, ports or prices from the event', async () => {
		await renderPage();
		await fireEvent.change(await screen.findByLabelText('Seria'), { target: { value: 'none' } });
		await pick('13');

		for (const label of [
			'Data zaokrętowania',
			'Data wyokrętowania',
			'Port wyjścia',
			'Port docelowy',
			'Cena dla gości / os.',
			'Kaucja zwrotna / os.'
		]) {
			expect(screen.getByLabelText(label)).not.toHaveValue();
		}
		expect(screen.getByLabelText('Waluta')).toHaveValue('PLN');
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
		expect(data.cruises.map((c) => c.id)).toEqual([13, 20]);
		expect(data.series.map((s) => s.id)).toEqual([2]);
		expect(data.loadErrors).toEqual([]);
	});

	it.each(['', '?cruise_id=abc', '?cruise_id=-3', '?cruise_id=1.5'])(
		'preselects no event for "%s"',
		async (query) => {
			expect((await runLoad(`http://localhost/trips/new${query}`)).cruiseId).toBe(0);
		}
	);

	it('names a failed event list instead of showing no events', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(listCruises).mockRejectedValue(new Error('down'));

		const data = await runLoad();

		expect(data.cruises).toEqual([]);
		expect(data.loadErrors).toEqual(['lista wydarzeń']);
	});

	it('names a failed series list', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(listSeries).mockRejectedValue(new Error('down'));

		const data = await runLoad();

		expect(data.series).toEqual([]);
		expect(data.loadErrors).toEqual(['lista serii']);
	});
});

describe('new trip page same-route navigation', () => {
	beforeEach(() => {
		fn(listCruises).mockResolvedValue([
			makeCruise({ id: 13, name: 'NNE', countries: 'Finlandia' }),
			makeCruise({ id: 14, name: 'WRM', countries: 'Polska' })
		]);
	});

	it('starts a fresh draft for the second event', async () => {
		const view = await renderPage('http://localhost/trips/new?cruise_id=13');
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), { target: { value: 'Etap 1' } });
		await fireEvent.input(screen.getByLabelText('Kraje'), { target: { value: 'Szwecja' } });

		await view.rerender({ data: await runLoad('http://localhost/trips/new?cruise_id=14') });

		expect(screen.getByLabelText('Edycja *')).toHaveValue('14');
		expect(screen.getByLabelText('Kraje')).toHaveValue('Polska');
		expect(screen.getByLabelText('Nazwa rejsu *')).toHaveValue('');
		expect(screen.getByRole('link', { name: 'Anuluj' })).toHaveAttribute('href', '/cruises/14');
	});

	it("clears the first draft's error", async () => {
		const view = await renderPage();
		await nameTrip();
		await fireEvent.change(screen.getByLabelText('Seria'), { target: { value: 'none' } });
		await submit();
		expect(await screen.findByText('Wybierz edycję, do której należy rejs')).toBeInTheDocument();

		await view.rerender({ data: await runLoad('http://localhost/trips/new?cruise_id=14') });

		expect(screen.queryByText('Wybierz edycję, do której należy rejs')).not.toBeInTheDocument();
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
		fn(listCrew).mockResolvedValue([
			makePerson({ id: 40, full_name: 'Kasia Admin', user_id: 1, has_account: true })
		]);
		const view = await renderPage('http://localhost/trips/new?cruise_id=13');
		await screen.findByRole('option', { name: 'Kasia Admin' });
		await fireEvent.change(screen.getByLabelText('Kapitan'), { target: { value: '0' } });

		await view.rerender({ data: await runLoad('http://localhost/trips/new?cruise_id=14') });

		await waitFor(() => expect(screen.getByLabelText('Kapitan')).toHaveValue('40'));
	});
});
