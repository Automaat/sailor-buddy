import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { makeCruise, makeTrip } from '#lib/test-utils.js';

const pageState = vi.hoisted(() => ({ url: new URL('http://localhost/trips/new') }));

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: pageState }));
vi.mock('#lib/stores/auth.svelte.js', () => ({ auth: { isAdmin: true, user: { id: 1, role: 'admin' } } }));
vi.mock('#lib/api/routes.js', () => ({ listCruises: vi.fn(), createTrip: vi.fn(), listMembers: vi.fn() }));

import { listCruises, createTrip, listMembers } from '#lib/api/routes.js';
import { goto } from '$app/navigation';
import NewTripPage from './+page.svelte';

const fn = (f: unknown) => f as ReturnType<typeof vi.fn>;

beforeEach(() => {
	vi.clearAllMocks();
	pageState.url = new URL('http://localhost/trips/new');
	fn(listMembers).mockResolvedValue([]);
	fn(listCruises).mockResolvedValue([makeCruise({ id: 13, name: 'NNE 2025', start_port: 'Turku' })]);
	fn(createTrip).mockResolvedValue(makeTrip({ id: 77 }));
});

describe('new trip page', () => {
	it('refuses to submit without a cruise', async () => {
		render(NewTripPage);
		await fireEvent.input(await screen.findByLabelText('Nazwa rejsu *'), { target: { value: 'Etap 1' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Zaplanuj rejs' }));

		expect(await screen.findByText('Wybierz wydarzenie, do którego należy rejs')).toBeInTheDocument();
		expect(createTrip).not.toHaveBeenCalled();
	});

	it('preselects and prefills the cruise from the URL and creates the leg in it', async () => {
		pageState.url = new URL('http://localhost/trips/new?cruise_id=13');
		render(NewTripPage);
		await waitFor(() => expect(screen.getByLabelText('Wydarzenie klubu *')).toHaveValue('13'));
		expect(screen.getByLabelText('Port wyjścia')).toHaveValue('Turku');
		await fireEvent.input(screen.getByLabelText('Nazwa rejsu *'), { target: { value: 'Etap 1' } });

		await fireEvent.click(screen.getByRole('button', { name: 'Zaplanuj rejs' }));

		await waitFor(() => expect(createTrip).toHaveBeenCalled());
		expect(fn(createTrip).mock.calls[0][0]).toMatchObject({ cruise_id: 13, name: 'Etap 1' });
		expect(goto).toHaveBeenCalledWith('/trips/77');
	});
});
