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
	createVoyage: vi.fn(),
	listMembers: vi.fn()
}));

import { listCruises, listYachts, createVoyage, listMembers } from '#lib/api/routes.js';
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
});
