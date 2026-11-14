import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import type { Cruise } from '#lib/api/types.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/api/routes.js', () => ({
	getCruise: vi.fn(),
	getSeries: vi.fn(),
	deleteCruise: vi.fn(),
	listCruiseTrips: vi.fn(),
	listCruiseVoyages: vi.fn(),
	listCruiseEnrollments: vi.fn(),
	listCruiseTripEnrollments: vi.fn(),
	listCruiseLegSignUps: vi.fn(),
	signUpForTrip: vi.fn(),
	updateTripSignUpNote: vi.fn(),
	withdrawTripSignUp: vi.fn(),
	generateCruiseEnrollToken: vi.fn(),
	clearCruiseEnrollToken: vi.fn(),
	updateCruiseEnrollmentStatus: vi.fn(),
	assignCruiseEnrollmentTrip: vi.fn(),
	deleteCruiseEnrollment: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: false, user: { id: 2, role: 'member' } }
}));

import { fireEvent, waitFor, within } from '@testing-library/svelte';
import { goto } from '$app/navigation';
import { ApiError } from '#lib/api/client.js';
import { auth } from '#lib/stores/auth.svelte.js';
import {
	getSeries,
	deleteCruise,
	generateCruiseEnrollToken,
	clearCruiseEnrollToken,
	updateCruiseEnrollmentStatus,
	assignCruiseEnrollmentTrip,
	deleteCruiseEnrollment
} from '#lib/api/routes.js';

import {
	getCruise,
	listCruiseTrips,
	listCruiseVoyages,
	listCruiseEnrollments,
	listCruiseTripEnrollments,
	listCruiseLegSignUps,
	signUpForTrip,
	updateTripSignUpNote,
	withdrawTripSignUp
} from '#lib/api/routes.js';
import CruiseDetailPage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

function runLoad(id = '7') {
	return load({ params: { id } } as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

// renderPage runs the route's load function and renders the page with its
// data, the way SvelteKit does on navigation.
async function renderPage() {
	return render(CruiseDetailPage, { props: { data: await runLoad() } });
}

const mockGet = getCruise as unknown as ReturnType<typeof vi.fn>;

function cruise(overrides: Partial<Cruise> = {}): Cruise {
	return {
		id: 7,
		name: 'Sycylia 2026',
		kind: 'sea',
		currency: 'PLN',
		included: [],
		not_included: [],
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

const mockAuth = auth as unknown as { isAdmin: boolean; user: Record<string, unknown> };
const fn = (f: unknown) => f as ReturnType<typeof vi.fn>;

beforeEach(() => {
	mockAuth.isAdmin = false;
	mockAuth.user = { id: 2, role: 'member', membership: 'czlonek' };
	vi.mocked(goto).mockReset();
	for (const f of [
		getSeries,
		deleteCruise,
		generateCruiseEnrollToken,
		clearCruiseEnrollToken,
		updateCruiseEnrollmentStatus,
		assignCruiseEnrollmentTrip,
		deleteCruiseEnrollment
	]) {
		fn(f).mockReset().mockResolvedValue(undefined);
	}
	mockGet.mockReset();
	(listCruiseTrips as unknown as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([]);
	(listCruiseVoyages as unknown as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([]);
	(listCruiseEnrollments as unknown as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([]);
	for (const f of [listCruiseTripEnrollments, listCruiseLegSignUps])
		fn(f).mockReset().mockResolvedValue([]);
	for (const f of [signUpForTrip, updateTripSignUpNote, withdrawTripSignUp]) fn(f).mockReset();
});

describe('cruise page logistics', () => {
	it('shows travel notes, minimum age and the group chat link', async () => {
		mockGet.mockResolvedValue(
			cruise({
				travel_notes: 'Tanie loty do Katanii',
				min_age: 18,
				group_chat_url: 'https://discord.gg/sycylia'
			})
		);
		await renderPage();

		expect(await screen.findByText('Informacje dla załogi')).toBeInTheDocument();
		expect(screen.getByText('Tanie loty do Katanii')).toBeInTheDocument();
		expect(screen.getByText('18+')).toBeInTheDocument();
		const link = screen.getByRole('link', { name: 'Czat grupy' });
		expect(link).toHaveAttribute('href', 'https://discord.gg/sycylia');
		expect(link).toHaveAttribute('target', '_blank');
		expect(link).toHaveAttribute('rel', 'noopener noreferrer');
	});

	it('shows a minimum age of zero', async () => {
		mockGet.mockResolvedValue(cruise({ min_age: 0 }));
		await renderPage();

		expect(await screen.findByText('0+')).toBeInTheDocument();
	});

	it('renders as before when no logistics are set', async () => {
		mockGet.mockResolvedValue(cruise());
		await renderPage();

		expect(await screen.findByRole('heading', { name: 'Sycylia 2026' })).toBeInTheDocument();
		expect(screen.queryByText('Informacje dla załogi')).not.toBeInTheDocument();
		expect(screen.queryByTestId('cruise-logistics')).not.toBeInTheDocument();
		expect(screen.queryByRole('link', { name: 'Czat grupy' })).not.toBeInTheDocument();
		expect(screen.getByText('Planowane etapy (0)')).toBeInTheDocument();
		expect(screen.queryByTestId('admin-sign-ups')).not.toBeInTheDocument();
	});
});

describe('cruise page logistics edge cases', () => {
	it('shows no logistics card when the only field is a non-http link', async () => {
		mockGet.mockResolvedValue(cruise({ group_chat_url: 'https://chat.example.com:99999/g' }));
		await renderPage();

		await screen.findByRole('heading', { name: 'Sycylia 2026' });
		expect(screen.queryByText('Informacje dla załogi')).not.toBeInTheDocument();
	});
});

const enrollmentRow = {
	id: 31,
	cruise_id: 7,
	user_id: 4,
	user_name: 'Ala Kowalska',
	user_email: 'ala@example.com',
	user_phone: '+48 600 100 200',
	user_patent_type: 'jachtowy_sternik_morski',
	user_patent_number: 'JSM-7',
	status: 'accepted',
	note: 'Mogę być kukiem',
	trip_id: 50,
	trip_name: 'Jacht A',
	created_at: '2026-01-01T00:00:00Z',
	updated_at: '2026-01-01T00:00:00Z'
};

function tripRow(id: number, name: string, status: string) {
	return {
		id,
		name,
		status,
		captain_name: 'Jan',
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z'
	};
}

describe('cruise page admin flows', () => {
	beforeEach(() => {
		mockAuth.isAdmin = true;
		mockGet.mockResolvedValue(cruise({ series_id: 3, edition: '2026', enroll_token: 'tok' }));
		fn(getSeries).mockResolvedValue({ id: 3, name: 'May I Sail' });
		fn(listCruiseTrips).mockResolvedValue([
			tripRow(50, 'Jacht A', 'planned'),
			tripRow(51, 'Jacht B', 'cancelled')
		]);
		fn(listCruiseVoyages).mockResolvedValue([{ id: 9, name: 'Rejs 2025', miles: 120.4, days: 7 }]);
		fn(listCruiseEnrollments).mockResolvedValue([
			enrollmentRow,
			{
				...enrollmentRow,
				id: 32,
				user_name: undefined,
				status: 'pending',
				note: undefined,
				trip_id: undefined,
				trip_name: undefined
			}
		]);
	});

	it('renders series, enrollments, trips and voyages', async () => {
		await renderPage();

		expect(await screen.findByRole('link', { name: 'May I Sail' })).toHaveAttribute(
			'href',
			'/series/3'
		);
		expect(screen.getByText('Ala Kowalska')).toBeInTheDocument();
		expect(screen.getByText('Nieznany')).toBeInTheDocument();
		expect(screen.getAllByRole('link', { name: '+48 600 100 200' })[0]).toHaveAttribute(
			'href',
			'tel:+48 600 100 200'
		);
		expect(screen.getAllByText('Jachtowy Sternik Morski · JSM-7')).toHaveLength(2);
		expect(screen.getByText('Mogę być kukiem')).toBeInTheDocument();
		expect(screen.getByText('Planowane etapy (2)')).toBeInTheDocument();
		expect(screen.getByText('Anulowany')).toBeInTheDocument();
		expect(screen.getByText('Zrealizowane etapy (1)')).toBeInTheDocument();
		expect(screen.getByText('120 Mm · 7 dni')).toBeInTheDocument();
	});

	it('changes enrollment status, assigns a trip and deletes an enrollment', async () => {
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		await renderPage();
		await screen.findByText('Ala Kowalska');

		await fireEvent.click(screen.getAllByRole('button', { name: 'Akceptuj' })[0]);
		await waitFor(() =>
			expect(updateCruiseEnrollmentStatus).toHaveBeenCalledWith(7, 32, 'accepted')
		);

		await fireEvent.click(screen.getAllByRole('button', { name: 'Rezerwa' })[0]);
		await waitFor(() =>
			expect(updateCruiseEnrollmentStatus).toHaveBeenCalledWith(7, 31, 'waitlisted')
		);

		const select = screen.getAllByRole('combobox')[0];
		await fireEvent.change(select, { target: { value: '' } });
		await waitFor(() =>
			expect(assignCruiseEnrollmentTrip).toHaveBeenCalledWith(7, expect.any(Number), undefined)
		);

		await fireEvent.click(screen.getAllByRole('button', { name: 'Odrzuć' })[0]);
		await waitFor(() =>
			expect(updateCruiseEnrollmentStatus).toHaveBeenCalledWith(7, expect.any(Number), 'rejected')
		);

		await fireEvent.click(screen.getAllByRole('button', { name: 'Usuń' }).at(-1)!);
		await waitFor(() => expect(deleteCruiseEnrollment).toHaveBeenCalledWith(7, 32));
		await waitFor(() => expect(screen.queryByText('Nieznany')).not.toBeInTheDocument());
	});

	it('explains a full leg when accepting a cruise enrollment is refused', async () => {
		fn(updateCruiseEnrollmentStatus).mockRejectedValueOnce(
			new ApiError(409, 'leg is full: all 1 places are taken')
		);
		vi.spyOn(console, 'error').mockImplementation(() => {});
		await renderPage();
		await screen.findByText('Ala Kowalska');

		await fireEvent.click(screen.getAllByRole('button', { name: 'Akceptuj' })[0]);

		expect(await screen.findByRole('alert')).toHaveTextContent('Etap jest pełny');
	});

	it('toggles enrollment off and on and copies the link', async () => {
		const writeText = vi.fn().mockResolvedValue(undefined);
		Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
		fn(generateCruiseEnrollToken).mockResolvedValue({ token: 'new' });
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Kopiuj link' }));
		expect(writeText).toHaveBeenCalledWith(`${window.location.origin}/enroll/tok`);

		await fireEvent.click(screen.getByRole('button', { name: 'Wyłącz zapisy' }));
		await waitFor(() => expect(clearCruiseEnrollToken).toHaveBeenCalledWith(7));
		await fireEvent.click(await screen.findByRole('button', { name: 'Włącz zapisy' }));
		await waitFor(() => expect(generateCruiseEnrollToken).toHaveBeenCalledWith(7));
		expect(await screen.findByRole('button', { name: 'Wyłącz zapisy' })).toBeInTheDocument();
	});

	it('refuses to delete a cruise that still has legs', async () => {
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		await renderPage();
		const header = (await screen.findByRole('heading', { name: 'Sycylia 2026' })).closest(
			'div'
		)!.parentElement!;

		await fireEvent.click(within(header).getByRole('button', { name: 'Usuń' }));

		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Nie można usunąć wydarzenia, które ma etapy'
		);
		expect(deleteCruise).not.toHaveBeenCalled();
	});

	it('shows the server error when deleting fails', async () => {
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		fn(listCruiseTrips).mockResolvedValue([]);
		fn(listCruiseVoyages).mockResolvedValue([]);
		fn(deleteCruise).mockRejectedValue(
			new Error('cruise still has trips or voyages; delete them first')
		);
		await renderPage();
		const header = (await screen.findByRole('heading', { name: 'Sycylia 2026' })).closest(
			'div'
		)!.parentElement!;

		await fireEvent.click(within(header).getByRole('button', { name: 'Usuń' }));

		expect(await screen.findByRole('alert')).toHaveTextContent('cruise still has trips or voyages');
		expect(goto).not.toHaveBeenCalled();
	});

	it('deletes the cruise after confirmation', async () => {
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		fn(listCruiseTrips).mockResolvedValue([]);
		fn(listCruiseVoyages).mockResolvedValue([]);
		await renderPage();
		const header = (await screen.findByRole('heading', { name: 'Sycylia 2026' })).closest(
			'div'
		)!.parentElement!;

		await fireEvent.click(within(header).getByRole('button', { name: 'Usuń' }));
		await waitFor(() => expect(deleteCruise).toHaveBeenCalledWith(7));
		expect(goto).toHaveBeenCalledWith('/cruises');
	});

	it('keeps the page usable when actions fail', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		fn(updateCruiseEnrollmentStatus).mockRejectedValue(new Error('x'));
		fn(assignCruiseEnrollmentTrip).mockRejectedValue(new Error('x'));
		fn(deleteCruiseEnrollment).mockRejectedValue(new Error('x'));
		fn(clearCruiseEnrollToken).mockRejectedValue(new Error('x'));
		await renderPage();
		await screen.findByText('Ala Kowalska');

		await fireEvent.click(screen.getAllByRole('button', { name: 'Akceptuj' })[0]);
		await fireEvent.change(screen.getAllByRole('combobox')[0], { target: { value: '51' } });
		await fireEvent.click(screen.getAllByRole('button', { name: 'Usuń' }).at(-1)!);
		await fireEvent.click(screen.getByRole('button', { name: 'Wyłącz zapisy' }));

		await waitFor(() => expect(console.error).toHaveBeenCalledTimes(4));
		expect(screen.getByText('Nieznany')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Wyłącz zapisy' })).toBeInTheDocument();
	});
});

// SvelteKit reuses this component when a link goes from one cruise to
// another; it rerenders with the new load data instead of remounting.
describe('cruise page same-route navigation', () => {
	beforeEach(() => {
		mockAuth.isAdmin = true;
		mockGet.mockImplementation(async (id: number) =>
			id === 7
				? cruise({ name: 'Sycylia 2026', enroll_token: 'tokA' })
				: cruise({ id, name: 'Chorwacja 2026', enroll_token: undefined })
		);
	});

	it('swaps to the second cruise and resets per-cruise state', async () => {
		fn(listCruiseTrips).mockImplementation(async (id: number) =>
			id === 7 ? [tripRow(50, 'Jacht A', 'planned')] : []
		);
		fn(listCruiseEnrollments).mockImplementation(async (id: number) =>
			id === 7 ? [enrollmentRow] : []
		);
		const view = await renderPage();
		expect(await screen.findByText(enrollmentRow.user_name as string)).toBeInTheDocument();
		const header = screen
			.getByRole('heading', { name: 'Sycylia 2026' })
			.closest('div')!.parentElement!;
		await fireEvent.click(within(header).getByRole('button', { name: 'Usuń' }));
		expect(screen.getByText(/Nie można usunąć wydarzenia/)).toBeInTheDocument();

		await view.rerender({ data: await runLoad('8') });

		expect(screen.getByRole('heading', { name: 'Chorwacja 2026' })).toBeInTheDocument();
		expect(screen.queryByText(/Nie można usunąć wydarzenia/)).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Włącz zapisy' })).toBeInTheDocument();
		await waitFor(() => expect(fn(listCruiseEnrollments)).toHaveBeenCalledWith(8));
		expect(screen.queryByText(enrollmentRow.user_name as string)).not.toBeInTheDocument();
	});

	it('drops an enroll-token reply for the cruise the page has left', async () => {
		mockGet.mockImplementation(async (id: number) =>
			cruise({ id, name: id === 7 ? 'Sycylia 2026' : 'Chorwacja 2026', enroll_token: undefined })
		);
		let answer: (v: { token: string }) => void = () => {};
		fn(generateCruiseEnrollToken).mockReturnValue(new Promise((r) => (answer = r)));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Włącz zapisy' }));

		await view.rerender({ data: await runLoad('8') });
		answer({ token: 'tokA' });
		await Promise.resolve();
		await Promise.resolve();

		expect(screen.getByRole('heading', { name: 'Chorwacja 2026' })).toBeInTheDocument();
		await waitFor(() =>
			expect(screen.getByRole('button', { name: 'Włącz zapisy' })).toBeInTheDocument()
		);
		expect(screen.queryByRole('button', { name: 'Wyłącz zapisy' })).not.toBeInTheDocument();
	});

	it('keeps the enroll toggle usable on a cruise reached while another toggles', async () => {
		mockGet.mockImplementation(async (id: number) =>
			cruise({ id, name: id === 7 ? 'Sycylia 2026' : 'Chorwacja 2026', enroll_token: undefined })
		);
		fn(generateCruiseEnrollToken).mockReturnValue(new Promise(() => {}));
		const view = await renderPage();
		const toggle = await screen.findByRole('button', { name: 'Włącz zapisy' });
		await fireEvent.click(toggle);
		expect(toggle).toBeDisabled();

		await view.rerender({ data: await runLoad('8') });

		expect(screen.getByRole('heading', { name: 'Chorwacja 2026' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Włącz zapisy' })).toBeEnabled();
	});
});

describe('cruise page load errors', () => {
	it('throws a 404 with the cruise copy for +error.svelte', async () => {
		mockGet.mockRejectedValue(new ApiError(404, 'not found'));

		await expect(runLoad()).rejects.toMatchObject({
			status: 404,
			body: {
				message: 'To wydarzenie nie istnieje lub nie masz do niego dostępu.',
				title: 'Nie znaleziono wydarzenia',
				backHref: '/cruises'
			}
		});
	});

	it('passes other failures on to +error.svelte', async () => {
		const boom = new Error('boom');
		mockGet.mockRejectedValue(boom);

		await expect(runLoad()).rejects.toBe(boom);
	});

	it('loads the cruise named by the URL', async () => {
		mockGet.mockResolvedValue(cruise({ id: 9 }));
		await runLoad('9');

		expect(mockGet).toHaveBeenCalledWith(9);
		expect(fn(listCruiseTrips)).toHaveBeenCalledWith(9);
	});

	it('names a failed leg list instead of showing no legs, and renders the rest', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		mockGet.mockResolvedValue(cruise());
		fn(listCruiseTrips).mockRejectedValue(new Error('500'));
		fn(listCruiseVoyages).mockResolvedValue([{ id: 9, name: 'Rejs 2025', miles: 120.4, days: 7 }]);
		await renderPage();

		expect(await screen.findByText(/Nie udało się wczytać: planowane etapy\./)).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'Sycylia 2026' })).toBeInTheDocument();
		expect(screen.getByText('Zrealizowane etapy (1)')).toBeInTheDocument();
	});

	it('shows no load banner when every section loads', async () => {
		mockGet.mockResolvedValue(cruise());
		await renderPage();

		await screen.findByRole('heading', { name: 'Sycylia 2026' });
		expect(screen.queryByText(/Nie udało się wczytać/)).not.toBeInTheDocument();
	});

	it('names failed admin sign-up lists for an admin', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		mockAuth.isAdmin = true;
		mockGet.mockResolvedValue(cruise());
		fn(listCruiseEnrollments).mockRejectedValue(new Error('500'));
		fn(listCruiseTripEnrollments).mockRejectedValue(new Error('500'));
		await renderPage();

		expect(
			await screen.findByText(/Nie udało się wczytać: zgłoszenia na wydarzenie, zapisy na etapy\./)
		).toBeInTheDocument();
	});
});

describe('cruise page price', () => {
	it.each([
		['czlonek', '820 PLN'],
		['sympatyk', '820 PLN'],
		['gosc', '870 PLN']
	] as const)('shows %s their price and the included lists', async (membership, want) => {
		mockAuth.user = { id: 2, role: 'member', membership };
		mockGet.mockResolvedValue(
			cruise({
				cost_per_person: 820,
				cost_per_person_non_member: 870,
				included: ['jacht', 'paliwo'],
				not_included: ['dojazd']
			})
		);
		await renderPage();
		expect(await screen.findByTestId('your-price')).toHaveTextContent(`Twoja cena: ${want}`);
		expect(screen.getByTestId('price-included')).toHaveTextContent('paliwo');
		expect(screen.getByTestId('price-not-included')).toHaveTextContent('dojazd');
	});

	it('hides the price card for an unpriced cruise', async () => {
		mockGet.mockResolvedValue(cruise({ cost_per_person: 0 }));
		await renderPage();
		expect(await screen.findByRole('heading', { name: 'Sycylia 2026' })).toBeInTheDocument();
		expect(screen.queryByRole('heading', { name: 'Cena' })).not.toBeInTheDocument();
	});
});

function legTrip(id: number, name: string, overrides: Record<string, unknown> = {}) {
	return {
		...tripRow(id, name, 'planned'),
		cruise_id: 7,
		currency: 'PLN',
		embark_date: '2026-07-01',
		disembark_date: '2026-07-08',
		max_crew: 8,
		cost_per_person: 2890,
		cost_per_person_non_member: 3100,
		...overrides
	};
}

function signUp(tripId: number, overrides: Record<string, unknown> = {}) {
	return { trip_id: tripId, open: true, accepted_count: 3, ...overrides };
}

function pending(tripId: number, overrides: Record<string, unknown> = {}) {
	return {
		id: 70 + tripId,
		trip_id: tripId,
		user_id: 2,
		status: 'pending',
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

function rowOf(name: string) {
	return screen.getAllByTestId('leg-row').find((r) => within(r).queryByRole('link', { name }))!;
}

describe('cruise page leg sign-up', () => {
	beforeEach(() => {
		mockGet.mockResolvedValue(cruise());
	});

	it('shows each leg with captain, dates, price and free places', async () => {
		fn(listCruiseTrips).mockResolvedValue([
			legTrip(50, 'Jacht A'),
			legTrip(51, 'Jacht B', { max_crew: undefined })
		]);
		fn(listCruiseLegSignUps).mockResolvedValue([signUp(50), signUp(51)]);
		await renderPage();
		await screen.findByRole('link', { name: 'Jacht A' });

		const a = rowOf('Jacht A');
		expect(a).toHaveTextContent('kpt. Jan');
		expect(a).toHaveTextContent('1–8 lip 2026 · 8 dni');
		expect(a).toHaveTextContent('2890 PLN');
		expect(within(a).getByTestId('free-places')).toHaveTextContent('wolne 5 z 8');
		expect(within(rowOf('Jacht B')).queryByTestId('free-places')).not.toBeInTheDocument();
	});

	it('shows a guest the non-member price', async () => {
		mockAuth.user = { id: 2, role: 'member', membership: 'gosc' };
		fn(listCruiseTrips).mockResolvedValue([legTrip(50, 'Jacht A')]);
		fn(listCruiseLegSignUps).mockResolvedValue([signUp(50)]);
		await renderPage();
		await screen.findByRole('link', { name: 'Jacht A' });
		expect(rowOf('Jacht A')).toHaveTextContent('3100 PLN');
	});

	it.each([
		['open, not signed up', signUp(50), 'button', 'Zapisz się: Jacht A'],
		['pending', signUp(50, { enrollment: pending(50) }), 'text', 'oczekujący'],
		[
			'accepted',
			signUp(50, { enrollment: pending(50, { status: 'accepted' }) }),
			'text',
			'zaakceptowany'
		],
		[
			'waitlisted',
			signUp(50, { enrollment: pending(50, { status: 'waitlisted' }), waitlist_position: 1 }),
			'text',
			'lista rezerwowa (1.)'
		],
		[
			'rejected',
			signUp(50, { enrollment: pending(50, { status: 'rejected' }) }),
			'text',
			'odrzucony'
		],
		['closed', signUp(50, { open: false }), 'text', 'Zapisy zamknięte']
	] as const)('shows the %s state', async (_name, state, kind, want) => {
		fn(listCruiseTrips).mockResolvedValue([legTrip(50, 'Jacht A')]);
		fn(listCruiseLegSignUps).mockResolvedValue([state]);
		await renderPage();
		await screen.findByRole('link', { name: 'Jacht A' });
		const row = rowOf('Jacht A');
		if (kind === 'button') {
			expect(within(row).getByRole('button', { name: want })).toBeEnabled();
		} else {
			expect(within(row).getByText(want)).toBeInTheDocument();
			expect(within(row).queryByRole('button', { name: /Zapisz się/ })).not.toBeInTheDocument();
		}
		const withdraw = within(row).queryByRole('button', { name: 'Wycofaj zapis: Jacht A' });
		// A member may withdraw while pending or on the waiting list.
		expect(withdraw !== null).toBe(want === 'oczekujący' || want === 'lista rezerwowa (1.)');
	});

	it('shows the sign-up deadline on the leg row', async () => {
		fn(listCruiseTrips).mockResolvedValue([
			legTrip(50, 'Jacht A', { sign_up_deadline: '2026-06-01' }),
			legTrip(51, 'Jacht B')
		]);
		fn(listCruiseLegSignUps).mockResolvedValue([signUp(50), signUp(51)]);
		await renderPage();
		await screen.findByRole('link', { name: 'Jacht A' });
		expect(within(rowOf('Jacht A')).getByTestId('sign-up-deadline')).toHaveTextContent(
			'Zapisy do 1 cze 2026'
		);
		expect(within(rowOf('Jacht B')).queryByTestId('sign-up-deadline')).not.toBeInTheDocument();
	});

	it('shows Zapisy zamknięte instead of the button once the deadline has passed', async () => {
		fn(listCruiseTrips).mockResolvedValue([
			legTrip(50, 'Jacht A', { sign_up_deadline: '2026-06-01' })
		]);
		fn(listCruiseLegSignUps).mockResolvedValue([signUp(50, { open: false })]);
		await renderPage();
		await screen.findByRole('link', { name: 'Jacht A' });
		const row = rowOf('Jacht A');
		expect(within(row).getByText('Zapisy zamknięte')).toBeInTheDocument();
		expect(within(row).getByTestId('sign-up-deadline')).toHaveTextContent('Zapisy do 1 cze 2026');
		expect(within(row).queryByRole('button', { name: /Zapisz się/ })).not.toBeInTheDocument();
	});

	it.each([
		['unpaid', 'Brak wpłaty'],
		['deposit', 'Zaliczka wpłacona'],
		['paid', 'Opłacone']
	] as const)('shows the %s payment status next to the sign-up status', async (payment, label) => {
		fn(listCruiseTrips).mockResolvedValue([legTrip(50, 'Jacht A')]);
		fn(listCruiseLegSignUps).mockResolvedValue([
			signUp(50, { enrollment: pending(50, { status: 'accepted', payment_status: payment }) })
		]);
		await renderPage();
		await screen.findByRole('link', { name: 'Jacht A' });
		const row = rowOf('Jacht A');
		expect(within(row).getByTestId('sign-up-status')).toHaveTextContent('zaakceptowany');
		expect(within(row).getByTestId('sign-up-payment')).toHaveTextContent(label);
	});

	it('shows a cruise-level enrollment assigned to the leg as the status, without withdraw or note', async () => {
		fn(listCruiseTrips).mockResolvedValue([legTrip(50, 'Jacht A')]);
		fn(listCruiseLegSignUps).mockResolvedValue([
			signUp(50, { enrollment: { ...pending(50), cruise_id: 7, status: 'pending' } })
		]);
		await renderPage();
		await screen.findByRole('link', { name: 'Jacht A' });
		const row = rowOf('Jacht A');
		expect(within(row).getByText('oczekujący')).toBeInTheDocument();
		expect(within(row).getByTestId('cruise-level')).toHaveTextContent('zapis na wydarzenie');
		expect(
			within(row).queryByRole('button', { name: /Zapisz się|Wycofaj|notatkę/ })
		).not.toBeInTheDocument();
	});

	it('shows a cancelled leg without a sign-up button', async () => {
		fn(listCruiseTrips).mockResolvedValue([legTrip(51, 'Jacht B', { status: 'cancelled' })]);
		fn(listCruiseLegSignUps).mockResolvedValue([signUp(51, { open: false })]);
		await renderPage();
		await screen.findByRole('link', { name: 'Jacht B' });
		const row = rowOf('Jacht B');
		expect(within(row).getByText('Anulowany')).toBeInTheDocument();
		expect(within(row).getByText('Etap odwołany')).toBeInTheDocument();
		expect(within(row).queryByRole('button', { name: /Zapisz się/ })).not.toBeInTheDocument();
	});

	it('signs up with one click and switches to pending without a reload', async () => {
		fn(listCruiseTrips).mockResolvedValue([legTrip(50, 'Jacht A')]);
		fn(listCruiseLegSignUps).mockResolvedValue([signUp(50)]);
		fn(signUpForTrip).mockResolvedValue({
			trip_id: 50,
			result: 'created',
			enrollment: pending(50)
		});
		await renderPage();
		const row = await screen.findByTestId('leg-row');
		expect(within(row).queryByRole('button', { name: 'Dodaj notatkę' })).not.toBeInTheDocument();
		expect(within(row).queryByRole('textbox')).not.toBeInTheDocument();

		await fireEvent.click(within(row).getByRole('button', { name: 'Zapisz się: Jacht A' }));

		await waitFor(() => expect(signUpForTrip).toHaveBeenCalledWith(50));
		expect(await within(row).findByText('oczekujący')).toBeInTheDocument();
		expect(within(row).queryByRole('button', { name: /Zapisz się/ })).not.toBeInTheDocument();
		expect(listCruiseLegSignUps).toHaveBeenCalledTimes(1);
	});

	it('adds a note to the organizer after signing up', async () => {
		fn(listCruiseTrips).mockResolvedValue([legTrip(50, 'Jacht A')]);
		fn(listCruiseLegSignUps).mockResolvedValue([signUp(50, { enrollment: pending(50) })]);
		fn(updateTripSignUpNote).mockResolvedValue(pending(50, { note: 'Mogę gotować' }));
		await renderPage();
		const row = await screen.findByTestId('leg-row');

		await fireEvent.click(within(row).getByRole('button', { name: 'Dodaj notatkę' }));
		await fireEvent.input(within(row).getByLabelText('Notatka dla organizatora'), {
			target: { value: 'Mogę gotować' }
		});
		await fireEvent.click(within(row).getByRole('button', { name: 'Zapisz notatkę' }));

		await waitFor(() => expect(updateTripSignUpNote).toHaveBeenCalledWith(50, 'Mogę gotować'));
		expect(await within(row).findByTestId('sign-up-note')).toHaveTextContent('Mogę gotować');
		expect(within(row).getByRole('button', { name: 'Edytuj notatkę' })).toBeInTheDocument();
	});

	it('withdraws a pending sign-up from the same row', async () => {
		fn(listCruiseTrips).mockResolvedValue([legTrip(50, 'Jacht A')]);
		fn(listCruiseLegSignUps).mockResolvedValue([signUp(50, { enrollment: pending(50) })]);
		fn(withdrawTripSignUp).mockResolvedValue(undefined);
		await renderPage();
		const row = await screen.findByTestId('leg-row');

		await fireEvent.click(within(row).getByRole('button', { name: 'Wycofaj zapis: Jacht A' }));

		await waitFor(() => expect(withdrawTripSignUp).toHaveBeenCalledWith(50));
		expect(
			await within(row).findByRole('button', { name: 'Zapisz się: Jacht A' })
		).toBeInTheDocument();
		expect(within(row).queryByText('oczekujący')).not.toBeInTheDocument();
	});

	it('shows the server error when signing up fails', async () => {
		fn(listCruiseTrips).mockResolvedValue([legTrip(50, 'Jacht A')]);
		fn(listCruiseLegSignUps).mockResolvedValue([signUp(50)]);
		fn(signUpForTrip).mockRejectedValue(new Error('sign-up closed: the leg is not planned'));
		await renderPage();
		const row = await screen.findByTestId('leg-row');

		await fireEvent.click(within(row).getByRole('button', { name: 'Zapisz się: Jacht A' }));

		expect(await within(row).findByRole('alert')).toHaveTextContent('sign-up closed');
		expect(within(row).getByRole('button', { name: 'Zapisz się: Jacht A' })).toBeInTheDocument();
	});

	it('loads no admin sign-up data for a member', async () => {
		await renderPage();
		await screen.findByRole('heading', { name: 'Sycylia 2026' });
		expect(screen.queryByTestId('admin-sign-ups')).not.toBeInTheDocument();
		expect(listCruiseTripEnrollments).not.toHaveBeenCalled();
		expect(listCruiseEnrollments).not.toHaveBeenCalled();
	});
});

describe('cruise page admin sign-ups by leg', () => {
	beforeEach(() => {
		mockAuth.isAdmin = true;
		mockGet.mockResolvedValue(cruise({ enroll_token: 'tok' }));
		fn(listCruiseTrips).mockResolvedValue([legTrip(50, 'Jacht A'), legTrip(51, 'Jacht B')]);
		fn(listCruiseTripEnrollments).mockResolvedValue([
			{
				...pending(50),
				id: 1,
				user_id: 4,
				user_name: 'Ola Nowak',
				user_email: 'ola@example.com',
				note: 'koja dziobowa'
			},
			{
				...pending(51),
				id: 2,
				user_id: 5,
				user_name: '',
				user_email: 'piotr@example.com',
				status: 'accepted'
			}
		]);
		fn(listCruiseEnrollments).mockResolvedValue([
			enrollmentRow,
			{ ...enrollmentRow, id: 33, user_name: 'Ewa Stara', trip_id: undefined, trip_name: undefined }
		]);
	});

	it('groups sign-ups by leg with a link to each review page', async () => {
		await renderPage();
		const a = await screen.findByRole('region', { name: 'Zapisy: Jacht A' });
		const b = screen.getByRole('region', { name: 'Zapisy: Jacht B' });

		expect(within(a).getByRole('link', { name: 'Zarządzaj zapisami' })).toHaveAttribute(
			'href',
			'/trips/50/enrollments'
		);
		expect(within(b).getByRole('link', { name: 'Zarządzaj zapisami' })).toHaveAttribute(
			'href',
			'/trips/51/enrollments'
		);
		expect(within(a).getByText('Ola Nowak')).toBeInTheDocument();
		expect(within(a).getByText('koja dziobowa')).toBeInTheDocument();
		expect(within(a).getByText('Ala Kowalska')).toBeInTheDocument();
		expect(within(b).getByText('piotr@example.com')).toBeInTheDocument();
		expect(within(b).getByText('zaakceptowany')).toBeInTheDocument();
	});

	it('lists unassigned cruise enrollments under Bez etapu and assigns them', async () => {
		await renderPage();
		const none = await screen.findByRole('region', { name: 'Zapisy: Bez etapu' });
		expect(within(none).getByText('Ewa Stara')).toBeInTheDocument();
		expect(within(none).queryByText('Ala Kowalska')).not.toBeInTheDocument();

		await fireEvent.change(within(none).getByRole('combobox', { name: 'Etap:' }), {
			target: { value: '51' }
		});
		await waitFor(() => expect(assignCruiseEnrollmentTrip).toHaveBeenCalledWith(7, 33, 51));
	});

	it('keeps the share-link controls', async () => {
		await renderPage();
		const section = await screen.findByTestId('admin-sign-ups');
		expect(within(section).getByRole('button', { name: 'Wyłącz zapisy' })).toBeInTheDocument();
		expect(within(section).getByRole('button', { name: 'Kopiuj link' })).toBeInTheDocument();
	});
});

describe('cruise page past planned legs', () => {
	beforeEach(() => {
		vi.useFakeTimers({ toFake: ['Date'] });
		vi.setSystemTime(new Date('2026-11-13T12:00:00Z'));
		mockGet.mockResolvedValue(cruise({ disembark_date: '2026-08-09' }));
		fn(listCruiseTrips).mockResolvedValue([
			tripRow(50, 'Jacht A', 'planned'),
			{ ...tripRow(51, 'Jacht B', 'planned'), disembark_date: '2026-11-20' }
		]);
	});
	afterEach(() => {
		vi.useRealTimers();
	});

	it('flags a leg past the cruise end and prompts an admin to complete it', async () => {
		mockAuth.isAdmin = true;
		await renderPage();
		await screen.findByRole('link', { name: 'Jacht A' });

		const overdue = rowOf('Jacht A');
		expect(within(overdue).getByRole('link', { name: 'Jacht A' })).toHaveAttribute(
			'href',
			'/trips/50'
		);
		expect(within(overdue).getByText('Do rozliczenia')).toBeInTheDocument();
		expect(within(overdue).getByText('Zrealizuj →')).toBeInTheDocument();

		const ongoing = rowOf('Jacht B');
		expect(within(ongoing).getByText('Planowany')).toBeInTheDocument();
		expect(within(ongoing).queryByText('Zrealizuj →')).not.toBeInTheDocument();
	});

	it('shows the status but no prompt to a member', async () => {
		await renderPage();
		await screen.findByRole('link', { name: 'Jacht A' });

		expect(within(rowOf('Jacht A')).getByText('Do rozliczenia')).toBeInTheDocument();
		expect(screen.queryByText('Zrealizuj →')).not.toBeInTheDocument();
	});
});

describe('cruise page leg labels', () => {
	it('offers an admin to add a leg and shows the empty state', async () => {
		mockAuth.isAdmin = true;
		mockGet.mockResolvedValue(cruise());
		await renderPage();

		const add = await screen.findByRole('link', { name: '+ Dodaj etap' });
		expect(add).toHaveAttribute('href', '/trips/new?cruise_id=7');
		expect(screen.getByText('Brak etapów')).toBeInTheDocument();
		expect(screen.queryByText(/Dodaj jacht/)).not.toBeInTheDocument();
	});

	it('hides the add-leg link from a member', async () => {
		mockGet.mockResolvedValue(cruise());
		await renderPage();

		expect(await screen.findByText('Brak etapów')).toBeInTheDocument();
		expect(screen.queryByRole('link', { name: '+ Dodaj etap' })).not.toBeInTheDocument();
	});
});
