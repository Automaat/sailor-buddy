import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { makeCruise, makeTrip } from '#lib/test-utils.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/api/routes.js', () => ({
	getTrip: vi.fn(),
	deleteTrip: vi.fn(),
	listTripCrew: vi.fn(),
	removeTripCrew: vi.fn(),
	updateTripCrewRole: vi.fn(),
	cancelTrip: vi.fn(),
	completeTrip: vi.fn(),
	getCruise: vi.fn(),
	generateTripEnrollToken: vi.fn(),
	clearTripEnrollToken: vi.fn(),
	getTripSignUp: vi.fn(() => Promise.resolve(null)),
	signUpForTrip: vi.fn(),
	updateTripSignUpNote: vi.fn(),
	withdrawTripSignUp: vi.fn(),
	publishTrip: vi.fn(),
	unpublishTrip: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: false, user: { id: 2, role: 'member' } }
}));

import { auth } from '#lib/stores/auth.svelte.js';
import { goto } from '$app/navigation';
import {
	cancelTrip,
	deleteTrip,
	generateTripEnrollToken,
	getCruise,
	getTrip,
	getTripSignUp,
	listTripCrew,
	publishTrip,
	removeTripCrew,
	updateTripCrewRole,
	unpublishTrip,
	signUpForTrip,
	updateTripSignUpNote
} from '#lib/api/routes.js';
import { ApiError } from '#lib/api/client.js';
import TripDetailPage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

function runLoad(id = '50') {
	return load({ params: { id } } as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

// renderPage runs the route's load function and renders the page with its
// data, the way SvelteKit does on navigation.
async function renderPage() {
	return render(TripDetailPage, { props: { data: await runLoad() } });
}

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
});

afterEach(() => {
	vi.useRealTimers();
});

describe('trip page status', () => {
	it('flags a past planned leg and prompts an admin to complete it', async () => {
		mockAuth.isAdmin = true;
		serve(
			makeTrip({ id: 50, name: 'Jacht A', embark_date: '2026-08-01', disembark_date: '2026-08-09' })
		);
		await renderPage();

		expect(await screen.findByText('Do rozliczenia')).toBeInTheDocument();
		expect(screen.queryByText('Nadchodzący')).not.toBeInTheDocument();
		expect(screen.getByRole('status')).toHaveTextContent('czeka na rozliczenie');
		expect(screen.getByText('1–9 sie 2026 · 9 dni')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Zrealizuj teraz' }));
		expect(await screen.findByRole('heading', { name: 'Zrealizuj rejs' })).toBeInTheDocument();
	});

	it('keeps a leg without dates planned', async () => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, embark_date: undefined, disembark_date: undefined }));
		await renderPage();

		expect(await screen.findByText('Nadchodzący')).toBeInTheDocument();
		expect(screen.queryByText('Do rozliczenia')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Zrealizuj teraz' })).not.toBeInTheDocument();
	});

	it('shows the status but no prompt to a member', async () => {
		serve(makeTrip({ id: 50, disembark_date: '2026-08-09' }));
		await renderPage();

		expect(await screen.findByText('Do rozliczenia')).toBeInTheDocument();
		expect(screen.queryByRole('status')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /Zrealizuj/ })).not.toBeInTheDocument();
	});

	it.each([
		{
			name: 'in progress',
			trip: { embark_date: '2026-11-10', disembark_date: '2026-11-20' },
			want: 'Nadchodzący'
		},
		{
			name: 'ending today',
			trip: { embark_date: '2026-11-10', disembark_date: '2026-11-13' },
			want: 'Nadchodzący'
		},
		{
			name: 'cancelled after it ended',
			trip: { status: 'cancelled' as const, disembark_date: '2026-08-09' },
			want: 'Anulowany'
		}
	])('keeps a leg $name as $want with no prompt', async ({ trip, want }) => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, ...trip }));
		await renderPage();

		expect(await screen.findByText(want)).toBeInTheDocument();
		expect(screen.queryByText('Do rozliczenia')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Zrealizuj teraz' })).not.toBeInTheDocument();
	});
});

describe('trip page sign-up toggle', () => {
	it('keeps a leg closed after switching sign-ups on past its deadline', async () => {
		mockAuth.isAdmin = true;
		serve(
			makeTrip({
				id: 50,
				name: 'Jacht A',
				embark_date: '2027-01-10',
				sign_up_deadline: '2026-11-12'
			})
		);
		vi.mocked(getTripSignUp).mockResolvedValue({ trip_id: 50, open: false, accepted_count: 0 });
		vi.mocked(generateTripEnrollToken).mockResolvedValue({ token: 'tok' });
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Włącz zapisy' }));
		expect(await screen.findByRole('button', { name: 'Wyłącz zapisy' })).toBeInTheDocument();
		expect(screen.getByText('Zapisy zamknięte')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Zapisz się: Jacht A' })).not.toBeInTheDocument();
	});

	it('opens a leg when sign-ups are switched on before its deadline', async () => {
		mockAuth.isAdmin = true;
		serve(
			makeTrip({
				id: 50,
				name: 'Jacht A',
				embark_date: '2027-01-10',
				sign_up_deadline: '2026-11-13'
			})
		);
		vi.mocked(getTripSignUp).mockResolvedValue({ trip_id: 50, open: false, accepted_count: 0 });
		vi.mocked(generateTripEnrollToken).mockResolvedValue({ token: 'tok' });
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Włącz zapisy' }));
		expect(await screen.findByRole('button', { name: 'Zapisz się: Jacht A' })).toBeEnabled();
	});
});

describe('trip page actions', () => {
	it('keeps Zrealizuj and Edytuj visible and tucks Odwołaj etap and Usuń into a menu', async () => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, status: 'planned' }));
		await renderPage();

		expect(await screen.findByRole('button', { name: 'Zrealizuj' })).toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'Edytuj' })).toHaveAttribute('href', '/trips/50/edit');
		expect(screen.queryByRole('menuitem', { name: 'Odwołaj etap' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Anuluj' })).not.toBeInTheDocument();

		const toggle = screen.getByRole('button', { name: 'Więcej akcji' });
		await fireEvent.click(toggle);
		expect(toggle).toHaveAttribute('aria-expanded', 'true');
		const menu = screen.getByRole('menu');
		expect(
			within(menu)
				.getAllByRole('menuitem')
				.map((el) => el.textContent?.trim())
		).toEqual(['Odwołaj etap', 'Usuń']);
	});

	it('asks for confirmation before cancelling the leg', async () => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, status: 'planned' }));
		const cancelled = makeTrip({ id: 50, status: 'cancelled' });
		vi.mocked(cancelTrip).mockResolvedValue(cancelled);
		const confirmSpy = vi
			.spyOn(window, 'confirm')
			.mockReturnValueOnce(false)
			.mockReturnValueOnce(true);
		await renderPage();

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
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Więcej akcji' }));
		expect(screen.getAllByRole('menuitem').map((el) => el.textContent?.trim())).toEqual(['Usuń']);
		expect(screen.queryByRole('button', { name: 'Zrealizuj' })).not.toBeInTheDocument();
	});

	it('closes the menu on Escape and on an outside click', async () => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, status: 'planned' }));
		await renderPage();

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
		await renderPage();

		expect(await screen.findByRole('heading', { name: 'Jacht A' })).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Więcej akcji' })).not.toBeInTheDocument();
	});

	it('goes back to the event after the leg is deleted', async () => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, cruise_id: 1 }));
		vi.mocked(deleteTrip).mockReset().mockResolvedValue(undefined);
		vi.mocked(goto).mockReset();
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Więcej akcji' }));
		await fireEvent.click(screen.getByRole('menuitem', { name: 'Usuń' }));
		await waitFor(() => expect(goto).toHaveBeenCalledWith('/cruises/1'));
		expect(deleteTrip).toHaveBeenCalledWith(50);
		confirmSpy.mockRestore();
	});

	it('shows the error and stays on the leg when deletion fails', async () => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, name: 'Jacht A', cruise_id: 1 }));
		vi.mocked(deleteTrip).mockReset().mockRejectedValue(new ApiError(409, 'trip has crew'));
		vi.mocked(goto).mockReset();
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
		const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Więcej akcji' }));
		await fireEvent.click(screen.getByRole('menuitem', { name: 'Usuń' }));
		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Nie udało się usunąć etapu: trip has crew'
		);
		expect(screen.getByRole('heading', { name: 'Jacht A' })).toBeInTheDocument();
		expect(goto).not.toHaveBeenCalled();
		confirmSpy.mockRestore();
		errorSpy.mockRestore();
	});
});

describe('trip page crew removal', () => {
	const anna = { id: 1, crew_member_id: 3, full_name: 'Anna Nowak', role: 'Sternik' } as never;

	beforeEach(() => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, name: 'Jacht A', status: 'planned' }));
		vi.mocked(listTripCrew).mockResolvedValue([anna]);
	});

	it('removes a crew member after confirmation', async () => {
		vi.mocked(removeTripCrew).mockReset().mockResolvedValue(undefined);
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));
		await waitFor(() => expect(screen.queryByText('Anna Nowak')).not.toBeInTheDocument());
		expect(removeTripCrew).toHaveBeenCalledWith(50, 1);
		expect(screen.queryByRole('alert')).not.toBeInTheDocument();
		confirmSpy.mockRestore();
	});

	it('shows the error and keeps the crew member when removal fails', async () => {
		vi.mocked(removeTripCrew)
			.mockReset()
			.mockRejectedValue(new ApiError(409, 'crew member has a voyage'));
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
		const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));
		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Nie udało się usunąć załoganta: crew member has a voyage'
		);
		expect(screen.getByText('Anna Nowak')).toBeInTheDocument();
		confirmSpy.mockRestore();
		errorSpy.mockRestore();
	});

	it('clears a removal error when a retry succeeds', async () => {
		vi.mocked(removeTripCrew)
			.mockReset()
			.mockRejectedValueOnce(new Error('offline'))
			.mockResolvedValueOnce(undefined);
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
		const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));
		expect(await screen.findByRole('alert')).toHaveTextContent('offline');
		await fireEvent.click(screen.getByRole('button', { name: 'Usuń' }));

		await waitFor(() => expect(screen.queryByText('Anna Nowak')).not.toBeInTheDocument());
		expect(screen.queryByRole('alert')).not.toBeInTheDocument();
		confirmSpy.mockRestore();
		errorSpy.mockRestore();
	});
});

describe('trip page crew from sign-ups', () => {
	const anna = {
		id: 1,
		crew_member_id: 3,
		full_name: 'Anna Nowak',
		role: 'zalogant',
		created_at: '2026-01-01T00:00:00Z'
	};

	beforeEach(() => {
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, name: 'Jacht A', status: 'planned' }));
		vi.mocked(listTripCrew).mockResolvedValue([anna]);
		vi.mocked(updateTripCrewRole).mockReset();
	});

	it('offers no form to add crew by hand and points to the sign-ups', async () => {
		await renderPage();

		const hint = await screen.findByTestId('crew-hint');
		expect(hint).toHaveTextContent('Załoga powstaje z zaakceptowanych zapisów');
		const manage = within(hint).getByRole('link', { name: 'Zarządzaj zapisami' });
		expect(manage).toHaveAttribute('href', '/trips/50/enrollments');
		expect(screen.queryByLabelText('Załogant')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Dodaj' })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Usuń' })).toBeInTheDocument();
	});

	it('saves an edited role for one crew member', async () => {
		vi.mocked(updateTripCrewRole).mockResolvedValue({ ...anna, role: 'oficer' });
		await renderPage();

		const role = await screen.findByLabelText('Rola: Anna Nowak');
		expect(role).toHaveValue('zalogant');
		const save = screen.getByRole('button', { name: 'Zapisz rolę: Anna Nowak' });
		expect(save).toBeDisabled();
		await fireEvent.input(role, { target: { value: '  oficer ' } });
		expect(save).toBeEnabled();
		await fireEvent.click(save);

		await waitFor(() => expect(updateTripCrewRole).toHaveBeenCalledWith(50, 1, 'oficer'));
		await waitFor(() => expect(save).toBeDisabled());
		expect(role).toHaveValue('oficer');
		expect(screen.queryByRole('alert')).not.toBeInTheDocument();
	});

	it('keeps a blank role from being saved', async () => {
		await renderPage();

		const role = await screen.findByLabelText('Rola: Anna Nowak');
		await fireEvent.input(role, { target: { value: '   ' } });
		expect(screen.getByRole('button', { name: 'Zapisz rolę: Anna Nowak' })).toBeDisabled();
		expect(updateTripCrewRole).not.toHaveBeenCalled();
	});

	it('shows the error and keeps the draft when saving the role fails', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		vi.mocked(updateTripCrewRole).mockRejectedValue(new ApiError(404, 'crew assignment not found'));
		await renderPage();

		const role = await screen.findByLabelText('Rola: Anna Nowak');
		await fireEvent.input(role, { target: { value: 'oficer' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz rolę: Anna Nowak' }));

		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Nie udało się zmienić roli: crew assignment not found'
		);
		expect(role).toHaveValue('oficer');
	});

	it('shows the role without editing on a cancelled leg', async () => {
		serve(makeTrip({ id: 50, name: 'Jacht A', status: 'cancelled' }));
		await renderPage();

		expect(await screen.findByText('zalogant')).toBeInTheDocument();
		expect(screen.queryByLabelText('Rola: Anna Nowak')).not.toBeInTheDocument();
		expect(screen.queryByTestId('crew-hint')).not.toBeInTheDocument();
	});

	it('shows a member neither the crew nor the role editor', async () => {
		mockAuth.isAdmin = false;
		await renderPage();

		await screen.findByRole('heading', { name: 'Jacht A' });
		expect(screen.queryByLabelText('Rola: Anna Nowak')).not.toBeInTheDocument();
		expect(screen.queryByTestId('crew-hint')).not.toBeInTheDocument();
	});
});

describe('trip page load errors', () => {
	it('names a failed crew list instead of showing no crew, and renders the leg', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, name: 'Jacht A' }));
		vi.mocked(listTripCrew).mockRejectedValueOnce(new Error('500'));
		await renderPage();

		expect(await screen.findByText(/Nie udało się wczytać: załoga\./)).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'Jacht A' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Więcej akcji' })).toBeInTheDocument();
	});

	it('names the cruise when only the cruise fails to load', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		serve(makeTrip({ id: 50, name: 'Jacht A' }));
		vi.mocked(getCruise).mockRejectedValueOnce(new Error('500'));
		await renderPage();

		expect(await screen.findByText(/Nie udało się wczytać: wydarzenie\./)).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'Jacht A' })).toBeInTheDocument();
	});

	it('names no admin-only section for a member', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		serve(makeTrip({ id: 50, name: 'Jacht A' }));
		vi.mocked(listTripCrew).mockRejectedValueOnce(new Error('500'));
		await renderPage();

		await screen.findByRole('heading', { name: 'Jacht A' });
		await waitFor(() => expect(listTripCrew).toHaveBeenCalled());
		expect(screen.queryByText(/Nie udało się wczytać/)).not.toBeInTheDocument();
	});

	it('still loads the crew for an admin whose role is known only after mount', async () => {
		serve(makeTrip({ id: 50, name: 'Jacht A' }));
		vi.mocked(listTripCrew).mockClear();
		await renderPage();

		await screen.findByRole('heading', { name: 'Jacht A' });
		expect(listTripCrew).toHaveBeenCalledWith(50);
	});

	it('drops the sign-up failure once the refresh after cancelling succeeds', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		mockAuth.isAdmin = true;
		serve(makeTrip({ id: 50, name: 'Jacht A', status: 'planned' }));
		vi.mocked(getTripSignUp)
			.mockRejectedValueOnce(new Error('500'))
			.mockResolvedValueOnce({ trip_id: 50, open: false, accepted_count: 0 });
		vi.mocked(cancelTrip).mockResolvedValue(
			makeTrip({ id: 50, name: 'Jacht A', status: 'cancelled' })
		);
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
		await renderPage();

		expect(await screen.findByText(/Nie udało się wczytać: zapis na etap\./)).toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Więcej akcji' }));
		await fireEvent.click(screen.getByRole('menuitem', { name: 'Odwołaj etap' }));

		expect(await screen.findByText('Anulowany')).toBeInTheDocument();
		await waitFor(() =>
			expect(screen.queryByText(/Nie udało się wczytać/)).not.toBeInTheDocument()
		);
		confirmSpy.mockRestore();
	});

	it('shows no load banner when every section loads', async () => {
		serve(makeTrip({ id: 50, name: 'Jacht A' }));
		await renderPage();

		await screen.findByRole('heading', { name: 'Jacht A' });
		expect(screen.queryByText(/Nie udało się wczytać/)).not.toBeInTheDocument();
	});
});

describe('trip page same-route navigation', () => {
	beforeEach(() => {
		mockAuth.isAdmin = true;
		vi.mocked(getCruise).mockResolvedValue(makeCruise({ id: 1 }));
		vi.mocked(getTrip).mockImplementation(async (id: number) =>
			makeTrip({
				id,
				name: id === 50 ? 'Jacht A' : 'Jacht B',
				status: 'planned',
				enroll_token: id === 50 ? 'tokA' : undefined
			})
		);
	});

	it('swaps to the second trip with its own crew and enroll link', async () => {
		vi.mocked(listTripCrew).mockImplementation(async (id: number) =>
			id === 50
				? [{ id: 1, crew_member_id: 3, full_name: 'Anna Nowak', role: 'Sternik' } as never]
				: []
		);
		const view = await renderPage();
		expect(await screen.findByText('Anna Nowak')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Wyłącz zapisy' })).toBeInTheDocument();

		await view.rerender({ data: await runLoad('51') });

		expect(screen.getByRole('heading', { name: 'Jacht B' })).toBeInTheDocument();
		expect(screen.queryByText('Anna Nowak')).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Włącz zapisy' })).toBeEnabled();
	});

	it('keeps the enroll toggle usable on a trip reached while another toggles', async () => {
		vi.mocked(getTrip).mockImplementation(async (id: number) =>
			makeTrip({ id, name: id === 50 ? 'Jacht A' : 'Jacht B', status: 'planned' })
		);
		let answer: (v: { token: string }) => void = () => {};
		vi.mocked(generateTripEnrollToken).mockReturnValue(new Promise((r) => (answer = r)));
		const view = await renderPage();
		const toggle = await screen.findByRole('button', { name: 'Włącz zapisy' });
		await fireEvent.click(toggle);
		expect(toggle).toBeDisabled();

		await view.rerender({ data: await runLoad('51') });
		expect(screen.getByRole('button', { name: 'Włącz zapisy' })).toBeEnabled();
		answer({ token: 'tokA' });
		await Promise.resolve();
		await Promise.resolve();

		expect(screen.getByRole('heading', { name: 'Jacht B' })).toBeInTheDocument();
		await waitFor(() =>
			expect(screen.getByRole('button', { name: 'Włącz zapisy' })).toBeInTheDocument()
		);
		expect(screen.queryByRole('button', { name: 'Wyłącz zapisy' })).not.toBeInTheDocument();
	});

	it('drops a cancel that answers after a link moved to another trip', async () => {
		let answer: (t: ReturnType<typeof makeTrip>) => void = () => {};
		vi.mocked(cancelTrip).mockReturnValue(new Promise((r) => (answer = r)));
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Więcej akcji' }));
		await fireEvent.click(screen.getByRole('menuitem', { name: 'Odwołaj etap' }));

		await view.rerender({ data: await runLoad('51') });
		answer(makeTrip({ id: 50, name: 'Jacht A', status: 'cancelled' }));
		await Promise.resolve();
		await Promise.resolve();

		expect(screen.getByRole('heading', { name: 'Jacht B' })).toBeInTheDocument();
		expect(screen.queryByRole('heading', { name: 'Jacht A' })).not.toBeInTheDocument();
		expect(screen.queryByText('Anulowany')).not.toBeInTheDocument();
		confirmSpy.mockRestore();
	});

	it('drops a leg delete failure that answers after a link moved to another trip', async () => {
		let fail: (err: Error) => void = () => {};
		vi.mocked(deleteTrip)
			.mockReset()
			.mockReturnValue(new Promise((_, reject) => (fail = reject)));
		vi.mocked(goto).mockReset();
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
		const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Więcej akcji' }));
		await fireEvent.click(screen.getByRole('menuitem', { name: 'Usuń' }));

		await view.rerender({ data: await runLoad('51') });
		fail(new ApiError(409, 'trip has crew'));
		expect(deleteTrip).toHaveBeenCalledWith(50);
		await new Promise((r) => setTimeout(r, 0));

		expect(screen.getByRole('heading', { name: 'Jacht B' })).toBeInTheDocument();
		expect(screen.queryByRole('alert')).not.toBeInTheDocument();
		expect(goto).not.toHaveBeenCalled();
		confirmSpy.mockRestore();
		errorSpy.mockRestore();
	});

	it('drops a crew removal failure that answers after a link moved to another trip', async () => {
		vi.mocked(listTripCrew).mockImplementation(async (id: number) =>
			id === 50
				? [{ id: 1, crew_member_id: 3, full_name: 'Anna Nowak', role: 'Sternik' } as never]
				: []
		);
		let fail: (err: Error) => void = () => {};
		vi.mocked(removeTripCrew)
			.mockReset()
			.mockReturnValue(new Promise((_, reject) => (fail = reject)));
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
		const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));

		await view.rerender({ data: await runLoad('51') });
		fail(new ApiError(409, 'crew member has a voyage'));
		expect(removeTripCrew).toHaveBeenCalledWith(50, 1);
		await new Promise((r) => setTimeout(r, 0));

		expect(screen.getByRole('heading', { name: 'Jacht B' })).toBeInTheDocument();
		expect(screen.queryByRole('alert')).not.toBeInTheDocument();
		confirmSpy.mockRestore();
		errorSpy.mockRestore();
	});
});

describe('trip page same-route navigation: sign-up control', () => {
	const pending = (tripId: number, note?: string) =>
		({ id: tripId, trip_id: tripId, status: 'pending', payment_status: 'unpaid', note }) as never;

	beforeEach(() => {
		vi.mocked(getCruise).mockResolvedValue(makeCruise({ id: 1 }));
		vi.mocked(getTrip).mockImplementation(async (id: number) =>
			makeTrip({ id, name: id === 50 ? 'Jacht A' : 'Jacht B', status: 'planned' })
		);
	});

	it('drops a sign-up reply and its busy flag after a link moved to another trip', async () => {
		vi.mocked(getTripSignUp).mockImplementation(async (id: number) => ({
			trip_id: id,
			open: true,
			accepted_count: 0
		}));
		let answer: (v: never) => void = () => {};
		vi.mocked(signUpForTrip).mockReturnValue(new Promise((r) => (answer = r)));
		const view = await renderPage();
		const button = await screen.findByRole('button', { name: 'Zapisz się: Jacht A' });
		await fireEvent.click(button);
		expect(button).toBeDisabled();

		await view.rerender({ data: await runLoad('51') });
		expect(screen.getByRole('button', { name: 'Zapisz się: Jacht B' })).toBeEnabled();
		answer({ enrollment: pending(50), waitlist_position: undefined } as never);
		await Promise.resolve();
		await Promise.resolve();

		expect(screen.getByRole('heading', { name: 'Jacht B' })).toBeInTheDocument();
		expect(screen.queryByTestId('sign-up-status')).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Zapisz się: Jacht B' })).toBeEnabled();
	});

	it('clears a sign-up alert and a note draft from the previous trip', async () => {
		vi.mocked(getTripSignUp).mockImplementation(async (id: number) => ({
			trip_id: id,
			open: true,
			accepted_count: 1,
			enrollment: pending(id, id === 50 ? 'note A' : undefined)
		}));
		vi.mocked(updateTripSignUpNote).mockRejectedValue(new Error('notatka odrzucona'));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Edytuj notatkę' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz notatkę' }));
		expect(await screen.findByText('notatka odrzucona')).toBeInTheDocument();

		await view.rerender({ data: await runLoad('51') });

		expect(screen.getByRole('heading', { name: 'Jacht B' })).toBeInTheDocument();
		expect(screen.queryByText('notatka odrzucona')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Zapisz notatkę' })).not.toBeInTheDocument();
		expect(screen.queryByDisplayValue('note A')).not.toBeInTheDocument();
	});
});

describe('trip page load', () => {
	it('throws a 404 with the trip copy for +error.svelte', async () => {
		vi.mocked(getTrip).mockRejectedValue(new ApiError(404, 'not found'));

		await expect(runLoad()).rejects.toMatchObject({
			status: 404,
			body: {
				message: 'Ten rejs nie istnieje lub nie masz do niego dostępu.',
				title: 'Nie znaleziono rejsu',
				backHref: '/cruises'
			}
		});
	});

	it('passes other failures on to +error.svelte', async () => {
		const boom = new Error('boom');
		vi.mocked(getTrip).mockRejectedValue(boom);

		await expect(runLoad()).rejects.toBe(boom);
	});

	it('loads the trip named by the URL', async () => {
		serve(makeTrip({ id: 9 }));
		await runLoad('9');

		expect(getTrip).toHaveBeenCalledWith(9);
		expect(getTripSignUp).toHaveBeenCalledWith(9);
		expect(listTripCrew).toHaveBeenCalledWith(9);
	});
});

describe('trip page publication', () => {
	const at = '2026-03-01T10:00:00Z';
	const authState = auth as unknown as { isAdmin: boolean; user: Record<string, unknown> };
	const header = () => screen.getByRole('heading', { name: 'Jacht A' }).parentElement!;

	beforeEach(() => {
		authState.isAdmin = true;
		authState.user = { id: 1, role: 'admin', is_captain: false };
		vi.mocked(publishTrip).mockReset();
		vi.mocked(unpublishTrip).mockReset();
	});

	afterEach(() => {
		authState.user = { id: 2, role: 'member' };
		vi.restoreAllMocks();
	});

	it('publishes a hidden trip and drops the badge', async () => {
		serve(makeTrip({ id: 50, name: 'Jacht A' }), makeCruise({ id: 1, published_at: at }));
		vi.mocked(publishTrip).mockResolvedValue(
			makeTrip({ id: 50, name: 'Jacht A', published_at: at })
		);
		await renderPage();
		expect(within(header()).getByText('Ukryte')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Opublikuj' }));

		expect(publishTrip).toHaveBeenCalledWith(50);
		await waitFor(() => expect(within(header()).queryByText('Ukryte')).not.toBeInTheDocument());
		expect(screen.getByRole('button', { name: 'Ukryj' })).toBeInTheDocument();
	});

	it('clears the badge when publishing also publishes a standalone cruise', async () => {
		serve(makeTrip({ id: 50, name: 'Jacht A' }), makeCruise({ id: 1 }));
		vi.mocked(publishTrip).mockResolvedValue(
			makeTrip({ id: 50, name: 'Jacht A', published_at: at })
		);
		await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: 'Opublikuj' }));

		await waitFor(() => expect(within(header()).queryByText('Ukryte')).not.toBeInTheDocument());
	});

	it('keeps the badge while the series edition stays hidden', async () => {
		serve(makeTrip({ id: 50, name: 'Jacht A' }), makeCruise({ id: 1, series_id: 3 }));
		vi.mocked(publishTrip).mockResolvedValue(
			makeTrip({ id: 50, name: 'Jacht A', published_at: at })
		);
		await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: 'Opublikuj' }));

		expect(await screen.findByRole('button', { name: 'Ukryj' })).toBeInTheDocument();
		expect(within(header()).getByText('Ukryte')).toHaveAttribute(
			'title',
			'Ukryte, bo wydarzenie jest ukryte'
		);
	});

	it('hides a published trip', async () => {
		serve(
			makeTrip({ id: 50, name: 'Jacht A', published_at: at }),
			makeCruise({ id: 1, published_at: at })
		);
		vi.mocked(unpublishTrip).mockResolvedValue(makeTrip({ id: 50, name: 'Jacht A' }));
		await renderPage();
		expect(within(header()).queryByText('Ukryte')).not.toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Ukryj' }));

		expect(unpublishTrip).toHaveBeenCalledWith(50);
		expect(await within(header()).findByText('Ukryte')).toBeInTheDocument();
	});

	it('keeps the badge and shows the error when publishing fails', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		serve(makeTrip({ id: 50, name: 'Jacht A' }), makeCruise({ id: 1, published_at: at }));
		vi.mocked(publishTrip).mockRejectedValue(new ApiError(500, 'failed to publish trip'));
		await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: 'Opublikuj' }));

		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Nie udało się opublikować etapu: failed to publish trip'
		);
		expect(within(header()).getByText('Ukryte')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Opublikuj' })).toBeEnabled();
	});

	it.each([
		['publishing opens', 'Opublikuj', false, true],
		['hiding closes', 'Ukryj', true, false]
	] as const)('reloads the sign-up: %s it', async (_name, action, before, after) => {
		const published = action === 'Ukryj' ? { published_at: at } : {};
		serve(
			makeTrip({ id: 50, name: 'Jacht A', ...published }),
			makeCruise({ id: 1, published_at: at })
		);
		vi.mocked(publishTrip).mockResolvedValue(
			makeTrip({ id: 50, name: 'Jacht A', published_at: at })
		);
		vi.mocked(unpublishTrip).mockResolvedValue(makeTrip({ id: 50, name: 'Jacht A' }));
		vi.mocked(getTripSignUp)
			.mockResolvedValueOnce({ trip_id: 50, open: before, accepted_count: 0 })
			.mockResolvedValueOnce({ trip_id: 50, open: after, accepted_count: 0 });
		await renderPage();
		const mine = screen.getByTestId('my-sign-up');
		expect(within(mine).queryByText('Zapisy zamknięte') === null).toBe(before);

		await fireEvent.click(screen.getByRole('button', { name: action }));

		await waitFor(() => expect(within(mine).queryByText('Zapisy zamknięte') === null).toBe(after));
		expect(getTripSignUp).toHaveBeenLastCalledWith(50);
	});

	it('shows the captain group the badge but no publish controls', async () => {
		authState.isAdmin = false;
		authState.user = { id: 3, role: 'member', is_captain: true };
		serve(makeTrip({ id: 50, name: 'Jacht A' }), makeCruise({ id: 1, published_at: at }));
		await renderPage();

		expect(within(header()).getByText('Ukryte')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Opublikuj' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Ukryj' })).not.toBeInTheDocument();
	});

	it('shows a member neither the badge nor publish controls', async () => {
		authState.isAdmin = false;
		authState.user = { id: 2, role: 'member', is_captain: false };
		serve(makeTrip({ id: 50, name: 'Jacht A' }), makeCruise({ id: 1 }));
		await renderPage();

		expect(screen.queryByText('Ukryte')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Opublikuj' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Ukryj' })).not.toBeInTheDocument();
	});
});
