import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { makeCruise, makeVoyage } from '#lib/test-utils.js';

const pageState = vi.hoisted(() => ({ url: new URL('http://localhost/voyages/new') }));

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: pageState }));
vi.mock('#lib/stores/auth.svelte.js', () => ({ auth: { isAdmin: true, user: { id: 1, role: 'admin' } } }));
vi.mock('#lib/api/routes.js', () => ({
	listCruises: vi.fn(),
	listYachts: vi.fn(),
	createYacht: vi.fn(),
	createVoyage: vi.fn(),
	listMembers: vi.fn()
}));

import { listCruises, listYachts, createVoyage, createYacht, listMembers } from '#lib/api/routes.js';
import NewVoyagePage from './+page.svelte';

const fn = (f: unknown) => f as ReturnType<typeof vi.fn>;

beforeEach(() => {
	vi.clearAllMocks();
	pageState.url = new URL('http://localhost/voyages/new');
	fn(listMembers).mockResolvedValue([]);
	fn(listYachts).mockResolvedValue([]);
	fn(listCruises).mockResolvedValue([makeCruise({ id: 13, name: 'NNE 2025' })]);
	fn(createVoyage).mockResolvedValue(makeVoyage({ id: 8 }));
});

describe('new voyage page', () => {
	it('refuses to submit without a cruise', async () => {
		render(NewVoyagePage);
		await fireEvent.input(await screen.findByLabelText('Nazwa rejsu *'), { target: { value: 'Rejs 2019' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj rejs' }));

		expect(await screen.findByText('Wybierz wydarzenie, do którego należy rejs')).toBeInTheDocument();
		expect(createVoyage).not.toHaveBeenCalled();
	});

	it('logs the voyage into the cruise named in the URL', async () => {
		pageState.url = new URL('http://localhost/voyages/new?cruise_id=13');
		render(NewVoyagePage);
		await waitFor(() => expect(screen.getByLabelText('Wydarzenie klubu *')).toHaveValue('13'));
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), { target: { value: 'Rejs 2019' } });

		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj rejs' }));

		await waitFor(() => expect(createVoyage).toHaveBeenCalled());
		expect(fn(createVoyage).mock.calls[0][0]).toMatchObject({ cruise_id: 13, name: 'Rejs 2019' });
	});

	it('adds a charter yacht inline and logs it with the track link and card', async () => {
		pageState.url = new URL('http://localhost/voyages/new?cruise_id=13');
		fn(createYacht).mockResolvedValue({
			id: 21,
			name: 'Selma',
			yacht_type: 'Hanse 415',
			registration_no: 'POL-1',
			created_at: '',
			updated_at: ''
		});
		render(NewVoyagePage);
		await waitFor(() => expect(screen.getByLabelText('Wydarzenie klubu *')).toHaveValue('13'));
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), { target: { value: 'Rejs 2019' } });

		await fireEvent.click(screen.getByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.input(screen.getByLabelText('Typ jachtu'), { target: { value: 'Hanse 415' } });
		await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: 'Selma' } });
		await fireEvent.input(screen.getByLabelText('Numer rejestracyjny (opcjonalnie)'), { target: { value: 'POL-1' } });
		await fireEvent.keyDown(screen.getByLabelText('Nazwa jachtu *'), { key: 'Enter' });

		await waitFor(() => expect(screen.getByLabelText('Jacht')).toHaveValue('21'));
		expect(createYacht).toHaveBeenCalledWith({ name: 'Selma', yacht_type: 'Hanse 415', registration_no: 'POL-1' });
		expect(createVoyage).not.toHaveBeenCalled();
		expect(screen.getByRole('option', { name: 'Selma (Hanse 415)' })).toBeInTheDocument();

		await fireEvent.input(screen.getByLabelText('Link do śladu rejsu'), { target: { value: 'https://example.com/t' } });
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
		pageState.url = new URL('http://localhost/voyages/new?cruise_id=13');
		let resolveYacht!: (y: unknown) => void;
		fn(createYacht).mockReturnValue(new Promise((r) => (resolveYacht = r)));
		render(NewVoyagePage);
		await waitFor(() => expect(screen.getByLabelText('Wydarzenie klubu *')).toHaveValue('13'));
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), { target: { value: 'Rejs 2019' } });

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
		render(NewVoyagePage);
		await fireEvent.click(await screen.findByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));

		expect(await screen.findByText('Podaj nazwę jachtu')).toBeInTheDocument();
		expect(createYacht).not.toHaveBeenCalled();
	});
});
