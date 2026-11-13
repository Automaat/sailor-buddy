import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { makeCruise, makeTrip } from '#lib/test-utils.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: { params: { id: '50' } } }));
vi.mock('#lib/api/routes.js', () => ({
	getTrip: vi.fn(),
	deleteTrip: vi.fn(),
	listTripCrew: vi.fn(),
	assignTripCrew: vi.fn(),
	removeTripCrew: vi.fn(),
	cancelTrip: vi.fn(),
	completeTrip: vi.fn(),
	listAssignableCrew: vi.fn(),
	getCruise: vi.fn(),
	generateTripEnrollToken: vi.fn(),
	clearTripEnrollToken: vi.fn(),
	getTripSignUp: vi.fn(() => Promise.resolve(null)),
	signUpForTrip: vi.fn(),
	updateTripSignUpNote: vi.fn(),
	withdrawTripSignUp: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: false, user: { id: 2, role: 'member' } }
}));

import { auth } from '#lib/stores/auth.svelte.js';
import {
	cancelTrip,
	generateTripEnrollToken,
	getCruise,
	getTrip,
	getTripSignUp,
	listAssignableCrew,
	listTripCrew
} from '#lib/api/routes.js';
import TripDetailPage from './+page.svelte';

const mockAuth = auth as unknown as { isAdmin: boolean };

function serve(trip: ReturnType<typeof makeTrip>, cruise = makeCruise({ id: 1 })) {
	vi.mocked(getTrip).mockResolvedValue(trip);
	vi.mocked(getCruise).mockResolvedValue(cruise);
}

beforeEach(() => {
	vi.useFakeTimers({ toFake: ['Date'] });
	vi.setSystemTime(new Date('2026-11-13T12:00:00Z'));
	mockAuth.isAdmin = false;
	vi.mocked(listTripCrew).mockResolvedValue([]);
	vi.mocked(listAssignableCrew).mockResolvedValue([]);
});

afterEach(() => {
	vi.useRealTimers();
});

describe('trip page status', () => {
	it('flags a past planned leg and prompts an admin to complete it', async () => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, name: 'Jacht A', embark_date: '2026-08-01', disembark_date: '2026-08-09' }));
		render(TripDetailPage);

		expect(await screen.findByText('Do rozliczenia')).toBeInTheDocument();
		expect(screen.queryByText('Planowany')).not.toBeInTheDocument();
		expect(screen.getByRole('status')).toHaveTextContent('czeka na rozliczenie');

		await fireEvent.click(screen.getByRole('button', { name: 'Zrealizuj teraz' }));
		expect(await screen.findByRole('heading', { name: 'Zrealizuj rejs' })).toBeInTheDocument();
	});

	it('uses the cruise end when the leg has no dates', async () => {
		mockAuth.isAdmin = true;
		serve(
			makeTrip({ id: 50, embark_date: undefined, disembark_date: undefined }),
			makeCruise({ id: 1, disembark_date: '2026-08-09' })
		);
		render(TripDetailPage);

		expect(await screen.findByText('Do rozliczenia')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Zrealizuj teraz' })).toBeInTheDocument();
	});

	it('shows the status but no prompt to a member', async () => {
		serve(makeTrip({ id: 50, disembark_date: '2026-08-09' }));
		render(TripDetailPage);

		expect(await screen.findByText('Do rozliczenia')).toBeInTheDocument();
		expect(screen.queryByRole('status')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /Zrealizuj/ })).not.toBeInTheDocument();
	});

	it.each([
		{ name: 'in progress', trip: { embark_date: '2026-11-10', disembark_date: '2026-11-20' }, want: 'Planowany' },
		{ name: 'ending today', trip: { embark_date: '2026-11-10', disembark_date: '2026-11-13' }, want: 'Planowany' },
		{ name: 'cancelled after it ended', trip: { status: 'cancelled' as const, disembark_date: '2026-08-09' }, want: 'Anulowany' }
	])('keeps a leg $name as $want with no prompt', async ({ trip, want }) => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, ...trip }));
		render(TripDetailPage);

		expect(await screen.findByText(want)).toBeInTheDocument();
		expect(screen.queryByText('Do rozliczenia')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Zrealizuj teraz' })).not.toBeInTheDocument();
	});
});

describe('trip page sign-up toggle', () => {
	it('keeps a leg closed after switching sign-ups on past its deadline', async () => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, name: 'Jacht A', embark_date: '2027-01-10', sign_up_deadline: '2026-11-12' }));
		vi.mocked(getTripSignUp).mockResolvedValue({ trip_id: 50, open: false, accepted_count: 0 });
		vi.mocked(generateTripEnrollToken).mockResolvedValue({ token: 'tok' });
		render(TripDetailPage);

		await fireEvent.click(await screen.findByRole('button', { name: 'Włącz zapisy' }));
		expect(await screen.findByRole('button', { name: 'Wyłącz zapisy' })).toBeInTheDocument();
		expect(screen.getByText('Zapisy zamknięte')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Zapisz się: Jacht A' })).not.toBeInTheDocument();
	});

	it('opens a leg when sign-ups are switched on before its deadline', async () => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, name: 'Jacht A', embark_date: '2027-01-10', sign_up_deadline: '2026-11-13' }));
		vi.mocked(getTripSignUp).mockResolvedValue({ trip_id: 50, open: false, accepted_count: 0 });
		vi.mocked(generateTripEnrollToken).mockResolvedValue({ token: 'tok' });
		render(TripDetailPage);

		await fireEvent.click(await screen.findByRole('button', { name: 'Włącz zapisy' }));
		expect(await screen.findByRole('button', { name: 'Zapisz się: Jacht A' })).toBeEnabled();
	});
});

describe('trip page actions', () => {
	it('keeps Zrealizuj and Edytuj visible and tucks Odwołaj etap and Usuń into a menu', async () => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, status: 'planned' }));
		render(TripDetailPage);

		expect(await screen.findByRole('button', { name: 'Zrealizuj' })).toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'Edytuj' })).toHaveAttribute('href', '/trips/50/edit');
		expect(screen.queryByRole('menuitem', { name: 'Odwołaj etap' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Anuluj' })).not.toBeInTheDocument();

		const toggle = screen.getByRole('button', { name: 'Więcej akcji' });
		await fireEvent.click(toggle);
		expect(toggle).toHaveAttribute('aria-expanded', 'true');
		const menu = screen.getByRole('menu');
		expect(within(menu).getAllByRole('menuitem').map((el) => el.textContent?.trim())).toEqual([
			'Odwołaj etap',
			'Usuń'
		]);
	});

	it('asks for confirmation before cancelling the leg', async () => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, status: 'planned' }));
		const cancelled = makeTrip({ id: 50, status: 'cancelled' });
		vi.mocked(cancelTrip).mockResolvedValue(cancelled);
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValueOnce(false).mockReturnValueOnce(true);
		render(TripDetailPage);

		await fireEvent.click(await screen.findByRole('button', { name: 'Więcej akcji' }));
		await fireEvent.click(screen.getByRole('menuitem', { name: 'Odwołaj etap' }));
		expect(confirmSpy).toHaveBeenCalledWith('Odwołać ten etap?');
		expect(cancelTrip).not.toHaveBeenCalled();
		expect(screen.queryByRole('menu')).not.toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Więcej akcji' }));
		await fireEvent.click(screen.getByRole('menuitem', { name: 'Odwołaj etap' }));
		expect(cancelTrip).toHaveBeenCalledWith(50);
		expect(await screen.findByText('Anulowany')).toBeInTheDocument();
		confirmSpy.mockRestore();
	});

	it('offers only Usuń in the menu once the leg is cancelled', async () => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, status: 'cancelled' }));
		render(TripDetailPage);

		await fireEvent.click(await screen.findByRole('button', { name: 'Więcej akcji' }));
		expect(screen.getAllByRole('menuitem').map((el) => el.textContent?.trim())).toEqual(['Usuń']);
		expect(screen.queryByRole('button', { name: 'Zrealizuj' })).not.toBeInTheDocument();
	});

	it('closes the menu on Escape and on an outside click', async () => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, status: 'planned' }));
		render(TripDetailPage);

		const toggle = await screen.findByRole('button', { name: 'Więcej akcji' });
		await fireEvent.click(toggle);
		await fireEvent.keyDown(window, { key: 'Escape' });
		expect(screen.queryByRole('menu')).not.toBeInTheDocument();

		await fireEvent.click(toggle);
		await fireEvent.click(document.body);
		expect(screen.queryByRole('menu')).not.toBeInTheDocument();
	});

	it('shows no actions menu to a member', async () => {
		serve(makeTrip({ id: 50, name: 'Jacht A', status: 'planned' }));
		render(TripDetailPage);

		expect(await screen.findByRole('heading', { name: 'Jacht A' })).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Więcej akcji' })).not.toBeInTheDocument();
	});
});
