import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { makeCruise, makeVoyage } from '#lib/test-utils.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));
vi.mock('#lib/api/routes.js', () => ({
	listCruises: vi.fn(),
	listYachts: vi.fn(),
	createYacht: vi.fn(),
	createVoyage: vi.fn(),
	listMembers: vi.fn(),
	listCrew: vi.fn()
}));

import {
	listCruises,
	listYachts,
	createVoyage,
	createYacht,
	listMembers,
	listCrew
} from '#lib/api/routes.js';
import { goto } from '$app/navigation';
import NewVoyagePage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

function runLoad(href = 'http://localhost/voyages/new') {
	return load({ url: new URL(href) } as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

/** renderPage runs the route's load function and renders the page with its data, the way SvelteKit does on navigation. */
async function renderPage(href?: string) {
	return render(NewVoyagePage, { props: { data: await runLoad(href) } });
}

const fn = (f: unknown) => f as ReturnType<typeof vi.fn>;

beforeEach(() => {
	vi.clearAllMocks();
	fn(listMembers).mockResolvedValue([]);
	fn(listCrew).mockResolvedValue([]);
	fn(listYachts).mockResolvedValue([]);
	fn(listCruises).mockResolvedValue([makeCruise({ id: 13, name: 'NNE 2025' })]);
	fn(createVoyage).mockResolvedValue(makeVoyage({ id: 8 }));
});

describe('new voyage page', () => {
	it('refuses to submit without a cruise', async () => {
		await renderPage();
		await fireEvent.input(await screen.findByLabelText('Nazwa rejsu *'), {
			target: { value: 'Rejs 2019' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj rejs' }));

		expect(
			await screen.findByText('Wybierz wydarzenie, do którego należy rejs')
		).toBeInTheDocument();
		expect(createVoyage).not.toHaveBeenCalled();
	});

	it('logs the voyage into the cruise named in the URL', async () => {
		await renderPage('http://localhost/voyages/new?cruise_id=13');
		await waitFor(() => expect(screen.getByLabelText('Wydarzenie klubu *')).toHaveValue('13'));
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), {
			target: { value: 'Rejs 2019' }
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj rejs' }));

		await waitFor(() => expect(createVoyage).toHaveBeenCalled());
		expect(fn(createVoyage).mock.calls[0][0]).toMatchObject({ cruise_id: 13, name: 'Rejs 2019' });
	});

	it('adds a charter yacht inline and logs it with the track link and card', async () => {
		fn(createYacht).mockResolvedValue({
			id: 21,
			name: 'Selma',
			yacht_type: 'Hanse 415',
			registration_no: 'POL-1',
			created_at: '',
			updated_at: ''
		});
		await renderPage('http://localhost/voyages/new?cruise_id=13');
		await waitFor(() => expect(screen.getByLabelText('Wydarzenie klubu *')).toHaveValue('13'));
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), {
			target: { value: 'Rejs 2019' }
		});

		await fireEvent.click(screen.getByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.input(screen.getByLabelText('Typ jachtu'), { target: { value: 'Hanse 415' } });
		await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: 'Selma' } });
		await fireEvent.input(screen.getByLabelText('Numer rejestracyjny (opcjonalnie)'), {
			target: { value: 'POL-1' }
		});
		await fireEvent.keyDown(screen.getByLabelText('Nazwa jachtu *'), { key: 'Enter' });

		await waitFor(() => expect(screen.getByLabelText('Jacht')).toHaveValue('21'));
		expect(createYacht).toHaveBeenCalledWith({
			name: 'Selma',
			yacht_type: 'Hanse 415',
			registration_no: 'POL-1'
		});
		expect(createVoyage).not.toHaveBeenCalled();
		expect(screen.getByRole('option', { name: 'Selma (Hanse 415)' })).toBeInTheDocument();

		await fireEvent.input(screen.getByLabelText('Link do śladu rejsu'), {
			target: { value: 'https://example.com/t' }
		});
		await fireEvent.input(screen.getByLabelText('Karta rejsu'), { target: { value: 'w klubie' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj rejs' }));

		await waitFor(() => expect(createVoyage).toHaveBeenCalled());
		expect(fn(createVoyage).mock.calls[0][0]).toMatchObject({
			yacht_id: 21,
			track_url: 'https://example.com/t',
			voyage_card: 'w klubie'
		});
	});

	it('saves the voyage with the yacht still being added', async () => {
		let resolveYacht!: (y: unknown) => void;
		fn(createYacht).mockReturnValue(new Promise((r) => (resolveYacht = r)));
		await renderPage('http://localhost/voyages/new?cruise_id=13');
		await waitFor(() => expect(screen.getByLabelText('Wydarzenie klubu *')).toHaveValue('13'));
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), {
			target: { value: 'Rejs 2019' }
		});

		await fireEvent.click(screen.getByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: 'Selma' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj rejs' }));
		expect(createVoyage).not.toHaveBeenCalled();

		resolveYacht({ id: 21, name: 'Selma', created_at: '', updated_at: '' });
		await waitFor(() => expect(createVoyage).toHaveBeenCalledTimes(1));
		expect(fn(createVoyage).mock.calls[0][0]).toMatchObject({ yacht_id: 21 });
	});

	it('keeps the form open when the yacht name is missing', async () => {
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));

		expect(await screen.findByText('Podaj nazwę jachtu')).toBeInTheDocument();
		expect(createYacht).not.toHaveBeenCalled();
	});
});

describe('new voyage page load errors', () => {
	it('names the failed cruise list and still renders the form', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(listCruises).mockRejectedValue(new Error('down'));
		await renderPage();

		expect(await screen.findByText(/Nie udało się wczytać: lista wydarzeń\./)).toBeInTheDocument();
		expect(screen.getByLabelText('Nazwa rejsu *')).toBeInTheDocument();
	});

	it('names the failed yacht list and still offers the cruises', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(listYachts).mockRejectedValue(new Error('down'));
		await renderPage();

		expect(await screen.findByText(/Nie udało się wczytać: jachty\./)).toBeInTheDocument();
		expect(screen.getByRole('option', { name: 'NNE 2025' })).toBeInTheDocument();
	});
});

describe('new voyage page load', () => {
	it('reads the preselected event from ?cruise_id=', async () => {
		const data = await runLoad('http://localhost/voyages/new?cruise_id=13');

		expect(data.cruiseId).toBe(13);
		expect(data.loadErrors).toEqual([]);
	});

	it('names both failed lists instead of showing empty pickers', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(listYachts).mockRejectedValue(new Error('down'));
		fn(listCruises).mockRejectedValue(new Error('down'));

		const data = await runLoad();

		expect(data.yachts).toEqual([]);
		expect(data.cruises).toEqual([]);
		expect(data.loadErrors).toEqual(['jachty', 'lista wydarzeń']);
	});
});

describe('new voyage page same-route navigation', () => {
	beforeEach(() => {
		fn(listCruises).mockResolvedValue([
			makeCruise({ id: 13, name: 'NNE 2025' }),
			makeCruise({ id: 14, name: 'WRM 2025' })
		]);
	});

	it('starts a fresh form for the second event', async () => {
		const view = await renderPage('http://localhost/voyages/new?cruise_id=13');
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), {
			target: { value: 'Rejs 2019' }
		});

		await view.rerender({ data: await runLoad('http://localhost/voyages/new?cruise_id=14') });

		expect(screen.getByLabelText('Wydarzenie klubu *')).toHaveValue('14');
		expect(screen.getByLabelText('Nazwa rejsu *')).toHaveValue('');
		expect(screen.getByRole('link', { name: 'Anuluj' })).toHaveAttribute('href', '/cruises/14');
	});

	it("clears the first form's error", async () => {
		const view = await renderPage();
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), {
			target: { value: 'Rejs 2019' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj rejs' }));
		expect(
			await screen.findByText('Wybierz wydarzenie, do którego należy rejs')
		).toBeInTheDocument();

		await view.rerender({ data: await runLoad('http://localhost/voyages/new?cruise_id=14') });

		expect(
			screen.queryByText('Wybierz wydarzenie, do którego należy rejs')
		).not.toBeInTheDocument();
	});

	it('drops a create reply for the form the page has left', async () => {
		let answer: (v: unknown) => void = () => {};
		fn(createVoyage).mockReturnValueOnce(new Promise((resolve) => (answer = resolve)));
		const view = await renderPage('http://localhost/voyages/new?cruise_id=13');
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), {
			target: { value: 'Rejs 2019' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj rejs' }));
		expect(screen.getByRole('button', { name: 'Tworzenie...' })).toBeDisabled();

		await view.rerender({ data: await runLoad('http://localhost/voyages/new?cruise_id=14') });

		expect(screen.getByRole('button', { name: 'Dodaj rejs' })).toBeEnabled();
		answer(makeVoyage({ id: 8 }));
		await new Promise((r) => setTimeout(r, 0));
		expect(goto).not.toHaveBeenCalled();
	});

	it('does not pick a yacht still being added for the first form', async () => {
		let resolveYacht!: (y: unknown) => void;
		fn(createYacht).mockReturnValue(new Promise((r) => (resolveYacht = r)));
		const view = await renderPage('http://localhost/voyages/new?cruise_id=13');
		await fireEvent.click(screen.getByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: 'Selma' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));

		await view.rerender({ data: await runLoad('http://localhost/voyages/new?cruise_id=14') });
		resolveYacht({ id: 21, name: 'Selma', created_at: '', updated_at: '' });
		await new Promise((r) => setTimeout(r, 0));

		expect(screen.getByLabelText('Jacht')).toHaveValue('0');
	});
});
