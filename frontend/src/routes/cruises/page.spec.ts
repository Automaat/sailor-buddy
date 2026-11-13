import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import { makeCruise, makeTrip, makeVoyage } from '#lib/test-utils.js';

const pageState = vi.hoisted(() => ({ url: new URL('http://localhost/cruises') }));

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: pageState }));
vi.mock('#lib/api/client.js', () => ({ api: { list: vi.fn(), get: vi.fn() } }));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));

import { api } from '#lib/api/client.js';
import { goto } from '$app/navigation';
import { auth } from '#lib/stores/auth.svelte.js';
import RejsyPage from './+page.svelte';

const apiList = api.list as unknown as ReturnType<typeof vi.fn>;
const apiGet = api.get as unknown as ReturnType<typeof vi.fn>;
const mockGoto = goto as unknown as ReturnType<typeof vi.fn>;
const mockAuth = auth as unknown as { isAdmin: boolean };

function serve(data: { cruises?: unknown[]; trips?: unknown[]; voyages?: unknown[]; mine?: number[] }) {
	apiGet.mockImplementation((path: string) =>
		path === '/auth/me/cruises'
			? Promise.resolve({ cruise_ids: data.mine ?? [] })
			: Promise.reject(new Error(`unexpected ${path}`))
	);
	apiList.mockImplementation((path: string) => {
		if (path === '/cruises') return Promise.resolve(data.cruises ?? []);
		if (path === '/trips') return Promise.resolve(data.trips ?? []);
		if (path === '/voyages') return Promise.resolve(data.voyages ?? []);
		return Promise.reject(new Error(`unexpected ${path}`));
	});
}

beforeEach(() => {
	// The factories' default leg (1-7 Jun 2025) is still ahead on this clock.
	vi.useFakeTimers({ toFake: ['Date'] });
	vi.setSystemTime(new Date('2025-05-01T12:00:00Z'));
	apiList.mockReset();
	apiGet.mockReset();
	mockGoto.mockReset();
	serve({});
	vi.spyOn(console, 'error').mockImplementation(() => {});
	mockAuth.isAdmin = true;
	pageState.url = new URL('http://localhost/cruises');
});

afterEach(() => {
	vi.useRealTimers();
});

describe('rejsy page', () => {
	it('shows a loading message while data is being fetched', () => {
		apiList.mockReturnValue(new Promise(() => {}));
		render(RejsyPage);
		expect(screen.getByText('Wczytywanie...')).toBeInTheDocument();
	});

	it('loads cruises, trips and voyages', async () => {
		render(RejsyPage);
		await screen.findByText('Brak nadchodzących rejsów');
		expect(apiList).toHaveBeenCalledWith('/cruises');
		expect(apiList).toHaveBeenCalledWith('/trips');
		expect(apiList).toHaveBeenCalledWith('/voyages');
	});

	it('renders an upcoming cruise card with its legs and open enrollment', async () => {
		serve({
			cruises: [makeCruise({ id: 9, name: 'Regaty Klubowe', start_port: 'Gdynia', end_port: 'Sopot', max_crew: 12, enroll_token: 'abc' })],
			trips: [makeTrip({ id: 1, cruise_id: 9 }), makeTrip({ id: 2, cruise_id: 9 })]
		});
		render(RejsyPage);

		const card = (await screen.findByText('Regaty Klubowe')).closest('a');
		expect(card).toHaveAttribute('href', '/cruises/9');
		expect(card).toHaveTextContent('Planowany');
		expect(card).toHaveTextContent('etapy: 0/2 zrealizowane');
		expect(screen.getByText('max 12 os.')).toBeInTheDocument();
		expect(screen.getByText('Zapisy otwarte')).toBeInTheDocument();
	});

	it('moves a fully sailed cruise to the completed tab with its totals', async () => {
		serve({
			cruises: [makeCruise({ id: 3, name: 'NNE 2025', enroll_token: 'old' })],
			voyages: [makeVoyage({ cruise_id: 3, miles: 310.4, hours_total: 52.6 })]
		});
		render(RejsyPage);
		await screen.findByText('Brak nadchodzących rejsów');
		expect(screen.queryByText('NNE 2025')).not.toBeInTheDocument();

		await fireEvent.click(screen.getByRole('tab', { name: 'Zrealizowane' }));

		const card = screen.getByText('NNE 2025').closest('a');
		expect(card).toHaveTextContent('Zrealizowany');
		expect(card).toHaveTextContent('53h');
		expect(card).toHaveTextContent('310');
		expect(card).not.toHaveTextContent('Zapisy otwarte');
		expect(mockGoto).toHaveBeenCalledWith('/cruises?tab=completed', { replace: true, shallow: true });
	});

	it('opens on the tab named in the URL', async () => {
		pageState.url = new URL('http://localhost/cruises?tab=completed');
		serve({ cruises: [makeCruise({ id: 3, name: 'NNE 2025' })], voyages: [makeVoyage({ cruise_id: 3 })] });
		render(RejsyPage);

		expect(await screen.findByText('NNE 2025')).toBeInTheDocument();
		expect(screen.getByRole('tab', { name: 'Zrealizowane' })).toHaveAttribute('aria-selected', 'true');
	});

	it('flags a past cruise with a planned leg for completion and lists it as sailed', async () => {
		vi.setSystemTime(new Date('2026-11-13T12:00:00Z'));
		serve({
			cruises: [makeCruise({ id: 5, name: 'Bornholm 2026', embark_date: '2026-08-01', disembark_date: '2026-08-09' })],
			trips: [makeTrip({ cruise_id: 5, embark_date: undefined, disembark_date: undefined })]
		});
		render(RejsyPage);
		await screen.findByText('Brak nadchodzących rejsów');
		expect(screen.queryByText('Bornholm 2026')).not.toBeInTheDocument();

		await fireEvent.click(screen.getByRole('tab', { name: 'Zrealizowane' }));

		const card = screen.getByText('Bornholm 2026').closest('a');
		expect(card).toHaveTextContent('Do rozliczenia');
		expect(card).not.toHaveTextContent('Planowany');
	});

	it('lists cancelled cruises only under all', async () => {
		serve({ cruises: [makeCruise({ id: 4, name: 'Odwołany' })], trips: [makeTrip({ cruise_id: 4, status: 'cancelled' })] });
		render(RejsyPage);
		await screen.findByText('Brak nadchodzących rejsów');

		await fireEvent.click(screen.getByRole('tab', { name: 'Wszystkie' }));

		expect(screen.getByText('Odwołany').closest('a')).toHaveTextContent('Anulowany');
		expect(mockGoto).toHaveBeenCalledWith('/cruises?tab=all', { replace: true, shallow: true });
	});

	it('shows the create button for an admin only', async () => {
		render(RejsyPage);
		await screen.findByText('Brak nadchodzących rejsów');
		expect(screen.getByRole('link', { name: '+ Nowe wydarzenie' })).toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'Utwórz pierwsze wydarzenie' })).toBeInTheDocument();
	});

	it('hides the create button for a non-admin member', async () => {
		mockAuth.isAdmin = false;
		render(RejsyPage);
		await screen.findByText('Brak nadchodzących rejsów');
		expect(screen.queryByRole('link', { name: '+ Nowe wydarzenie' })).not.toBeInTheDocument();
	});

	it('falls back to the empty state when loading fails', async () => {
		apiList.mockRejectedValue(new Error('network down'));
		render(RejsyPage);
		expect(await screen.findByText('Brak nadchodzących rejsów')).toBeInTheDocument();
		expect(console.error).toHaveBeenCalled();
	});

	describe('kind filter', () => {
		const mixed = () =>
			serve({
				cruises: [
					makeCruise({ id: 1, name: 'Bałtyk', kind: 'sea' }),
					makeCruise({ id: 2, name: 'Mazury', kind: 'inland' })
				]
			});

		it('shows the kind on each card', async () => {
			mixed();
			render(RejsyPage);
			expect((await screen.findByText('Bałtyk')).closest('a')).toHaveTextContent('Morski');
			expect(screen.getByText('Mazury').closest('a')).toHaveTextContent('Śródlądowy');
		});

		it.each([
			['Morski', 'Bałtyk', 'Mazury'],
			['Śródlądowy', 'Mazury', 'Bałtyk']
		])('filters to %s cruises', async (label, shown, hidden) => {
			mixed();
			render(RejsyPage);
			await screen.findByText('Bałtyk');

			const button = screen.getByRole('button', { name: label });
			await fireEvent.click(button);

			expect(button).toHaveAttribute('aria-pressed', 'true');
			expect(screen.getByText(shown)).toBeInTheDocument();
			expect(screen.queryByText(hidden)).not.toBeInTheDocument();
		});

		it('shows a filtered empty state when no cruise has the selected kind', async () => {
			serve({ cruises: [makeCruise({ id: 1, name: 'Bałtyk', kind: 'sea' })] });
			render(RejsyPage);
			await screen.findByText('Bałtyk');

			await fireEvent.click(screen.getByRole('button', { name: 'Śródlądowy' }));

			expect(screen.getByText('Brak rejsów tego rodzaju')).toBeInTheDocument();
		});
	});

	describe('my cruises filter', () => {
		const cruises = [
			makeCruise({ id: 1, name: 'Mój rejs' }),
			makeCruise({ id: 2, name: 'Cudzy rejs' }),
			makeCruise({ id: 3, name: 'Mój stary rejs' })
		];
		const trips = [makeTrip({ id: 10, cruise_id: 1 }), makeTrip({ id: 11, cruise_id: 2 })];
		const voyages = [makeVoyage({ id: 20, cruise_id: 3 })];

		it('narrows the list to cruises the user took part in and keeps it in the URL', async () => {
			serve({ cruises, trips, voyages, mine: [1, 3] });
			render(RejsyPage);
			await screen.findByText('Cudzy rejs');

			const toggle = screen.getByRole('button', { name: 'Moje rejsy' });
			expect(toggle).toHaveAttribute('aria-pressed', 'false');
			await fireEvent.click(toggle);

			expect(toggle).toHaveAttribute('aria-pressed', 'true');
			expect(screen.getByText('Mój rejs')).toBeInTheDocument();
			expect(screen.queryByText('Cudzy rejs')).not.toBeInTheDocument();
			expect(mockGoto).toHaveBeenCalledWith('/cruises?mine=1', { replace: true, shallow: true });

			await fireEvent.click(screen.getByRole('tab', { name: 'Zrealizowane' }));
			expect(screen.getByText('Mój stary rejs')).toBeInTheDocument();
			expect(mockGoto).toHaveBeenLastCalledWith('/cruises?tab=completed&mine=1', { replace: true, shallow: true });
		});

		it('opens with the filter on from the URL and explains an empty result', async () => {
			pageState.url = new URL('http://localhost/cruises?mine=1');
			serve({ cruises, trips, voyages, mine: [] });
			render(RejsyPage);

			expect(await screen.findByText('Nie masz nadchodzących rejsów')).toBeInTheDocument();
			expect(screen.getByRole('button', { name: 'Moje rejsy' })).toHaveAttribute('aria-pressed', 'true');
		});

		it('still lists all cruises when loading the user\'s cruises fails', async () => {
			serve({ cruises, trips, voyages });
			apiGet.mockRejectedValue(new Error('boom'));
			render(RejsyPage);

			expect(await screen.findByText('Cudzy rejs')).toBeInTheDocument();
			expect(screen.getByText('Mój rejs')).toBeInTheDocument();
		});

		it('reports a failed load instead of claiming the user has no cruises', async () => {
			serve({ cruises, trips, voyages, mine: [1] });
			apiGet.mockRejectedValueOnce(new Error('boom'));
			render(RejsyPage);
			await screen.findByText('Cudzy rejs');

			await fireEvent.click(screen.getByRole('button', { name: 'Moje rejsy' }));
			expect(screen.getByRole('alert')).toHaveTextContent('Nie udało się wczytać Twoich rejsów');
			expect(screen.queryByText('Nie masz nadchodzących rejsów')).not.toBeInTheDocument();

			await fireEvent.click(screen.getByRole('button', { name: 'Spróbuj ponownie' }));
			expect(await screen.findByText('Mój rejs')).toBeInTheDocument();
			expect(screen.queryByText('Cudzy rejs')).not.toBeInTheDocument();
		});
	});
});
