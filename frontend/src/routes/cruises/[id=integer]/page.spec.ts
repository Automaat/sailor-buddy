import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import type { Cruise } from '#lib/api/types.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/api/routes.js', () => ({
	getCruise: vi.fn(),
	getSeries: vi.fn(),
	deleteCruise: vi.fn(),
	publishCruise: vi.fn(),
	unpublishCruise: vi.fn(),
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
	publishCruise,
	unpublishCruise,
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
import { makeSeries } from '#lib/test-utils.js';
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
		publishCruise,
		unpublishCruise,
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

describe('cruise page back link', () => {
	it.each([
		['the Rejsy list for a cruise without a series', undefined, null, 'Rejsy', '/rejsy'],
		[
			'its series for an edition',
			3,
			makeSeries({ id: 3, name: 'May I Sail' }),
			'May I Sail',
			'/rejsy/3'
		],
		['its series by id when the series fails to load', 3, null, 'Seria', '/rejsy/3']
	])('leads to %s', async (_, seriesId, row, label, href) => {
		mockGet.mockResolvedValue(cruise({ series_id: seriesId }));
		fn(getSeries).mockImplementation(() =>
			row ? Promise.resolve(row) : Promise.reject(new Error('down'))
		);
		vi.spyOn(console, 'error').mockImplementation(() => {});
		await renderPage();

		const back = await screen.findByTestId('back-link');
		expect(back).toHaveAttribute('href', href);
		expect(back).toHaveAccessibleName(label);
	});
});

describe('cruise page logistics', () => {
	it('shows travel notes and minimum age', async () => {
		mockGet.mockResolvedValue(cruise({ travel_notes: 'Tanie loty do Katanii', min_age: 18 }));
		await renderPage();

		expect(await screen.findByText('Informacje dla załogi')).toBeInTheDocument();
		expect(screen.getByText('Tanie loty do Katanii')).toBeInTheDocument();
		expect(screen.getByText('18+')).toBeInTheDocument();
	});

	it('shows a minimum age of zero', async () => {
		mockGet.mockResolvedValue(cruise({ min_age: 0 }));
		await renderPage();

		expect(await screen.findByText('0+')).toBeInTheDocument();
	});

	it('lets a long unbroken edition name wrap in the header', async () => {
		const long = 'X'.repeat(200);
		mockGet.mockResolvedValue(cruise({ name: long }));
		await renderPage();

		const heading = await screen.findByRole('heading', { name: long });
		expect(heading).toHaveClass('min-w-0', '[overflow-wrap:anywhere]');
		expect(heading.parentElement?.parentElement).toHaveClass('min-w-0');
	});

	it('lets a long unbroken captain name wrap in its leg tile', async () => {
		const long = 'W'.repeat(190);
		fn(listCruiseTrips).mockResolvedValue([legTrip(50, 'Jacht A', { captain_name: long })]);
		mockGet.mockResolvedValue(cruise());
		await renderPage();

		const meta = (await screen.findByText(`kpt. ${long}`)).parentElement!;
		expect(meta).toHaveClass('[overflow-wrap:anywhere]', '[&>*]:min-w-0');
	});

	it('lets long unbroken description, notes and price items wrap', async () => {
		const long = 'W'.repeat(180);
		mockGet.mockResolvedValue(
			cruise({ description: `D${long}`, travel_notes: `T${long}`, included: [`I${long}`] })
		);
		await renderPage();

		expect(await screen.findByText(`D${long}`)).toHaveClass('[overflow-wrap:anywhere]');
		expect(screen.getByText(`T${long}`)).toHaveClass('[overflow-wrap:anywhere]');
		expect(screen.getByRole('region', { name: 'Co obejmuje cena' })).toHaveClass(
			'[overflow-wrap:anywhere]'
		);
	});

	it('renders as before when no logistics are set', async () => {
		mockGet.mockResolvedValue(cruise());
		await renderPage();

		expect(await screen.findByRole('heading', { name: 'Sycylia 2026' })).toBeInTheDocument();
		expect(screen.queryByText('Informacje dla załogi')).not.toBeInTheDocument();
		expect(screen.queryByTestId('cruise-logistics')).not.toBeInTheDocument();
		expect(screen.getByText('Nadchodzące etapy (0)')).toBeInTheDocument();
		expect(screen.queryByTestId('admin-sign-ups')).not.toBeInTheDocument();
	});
});

describe('cruise page header', () => {
	it("shows the edition's countries", async () => {
		mockGet.mockResolvedValue(cruise({ countries: 'Włochy, Malta' }));
		await renderPage();

		await screen.findByRole('heading', { name: 'Sycylia 2026' });
		expect(screen.getByText('Włochy, Malta')).toBeInTheDocument();
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
			'/rejsy/3'
		);
		expect(screen.getByText('Ala Kowalska')).toBeInTheDocument();
		expect(screen.getByText('Nieznany')).toBeInTheDocument();
		expect(screen.getAllByRole('link', { name: '+48 600 100 200' })[0]).toHaveAttribute(
			'href',
			'tel:+48 600 100 200'
		);
		expect(screen.getAllByText('Jachtowy Sternik Morski · JSM-7')).toHaveLength(2);
		expect(screen.getByText('Mogę być kukiem')).toBeInTheDocument();
		expect(screen.getByText('Nadchodzące etapy (2)')).toBeInTheDocument();
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
		const header = await screen.findByTestId('cruise-header');

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
		const header = await screen.findByTestId('cruise-header');

		await fireEvent.click(within(header).getByRole('button', { name: 'Usuń' }));

		expect(await screen.findByRole('alert')).toHaveTextContent('cruise still has trips or voyages');
		expect(goto).not.toHaveBeenCalled();
	});

	it.each([
		['the Rejsy list for a cruise without a series', undefined, '/rejsy'],
		["its series' page for an edition", 3, '/rejsy/3']
	])('deletes the cruise after confirmation and opens %s', async (_, seriesId, href) => {
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		mockGet.mockResolvedValue(cruise({ series_id: seriesId }));
		fn(listCruiseTrips).mockResolvedValue([]);
		fn(listCruiseVoyages).mockResolvedValue([]);
		await renderPage();
		const header = await screen.findByTestId('cruise-header');

		await fireEvent.click(within(header).getByRole('button', { name: 'Usuń' }));
		await waitFor(() => expect(deleteCruise).toHaveBeenCalledWith(7));
		expect(goto).toHaveBeenCalledWith(href);
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
		const header = screen.getByTestId('cruise-header');
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
				backHref: '/rejsy'
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

		expect(
			await screen.findByText(/Nie udało się wczytać: nadchodzące etapy\./)
		).toBeInTheDocument();
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

describe('cruise page price coverage', () => {
	it('shows what the leg price covers without a cruise price', async () => {
		mockGet.mockResolvedValue(cruise({ included: ['jacht', 'paliwo'], not_included: ['dojazd'] }));
		await renderPage();

		const heading = await screen.findByRole('heading', { name: 'Co obejmuje cena etapu' });
		const card = heading.parentElement!;
		expect(within(card).getByTestId('price-included')).toHaveTextContent('paliwo');
		expect(within(card).getByTestId('price-not-included')).toHaveTextContent('dojazd');
		expect(screen.queryByRole('heading', { name: 'Cena' })).not.toBeInTheDocument();
		expect(screen.queryByTestId('your-price')).not.toBeInTheDocument();
	});

	it('hides the coverage card when both lists are empty', async () => {
		mockGet.mockResolvedValue(cruise());
		await renderPage();
		expect(await screen.findByRole('heading', { name: 'Sycylia 2026' })).toBeInTheDocument();
		expect(
			screen.queryByRole('heading', { name: 'Co obejmuje cena etapu' })
		).not.toBeInTheDocument();
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

describe('cruise page leg tiles', () => {
	let followed: (string | null)[];
	/**
	 * Records what SvelteKit's router would follow: the link holding a click's
	 * target unless the default was prevented. A click on the link's ::after,
	 * which covers the tile, targets the link itself.
	 */
	function follow(e: MouseEvent) {
		const link = e.defaultPrevented ? null : (e.target as Element).closest('a');
		followed.push(link?.getAttribute('href') ?? null);
		e.preventDefault();
	}

	beforeEach(() => {
		followed = [];
		document.addEventListener('click', follow);
		mockGet.mockResolvedValue(cruise());
		fn(listCruiseTrips).mockResolvedValue([legTrip(50, 'Jacht A')]);
		fn(listCruiseVoyages).mockResolvedValue([{ id: 9, name: 'Rejs 2025', miles: 120, days: 7 }]);
		fn(listCruiseLegSignUps).mockResolvedValue([signUp(50)]);
	});
	afterEach(() => {
		document.removeEventListener('click', follow);
	});

	it.each([
		{ kind: 'leg', testId: 'leg-row', name: 'Jacht A', href: '/trips/50' },
		{ kind: 'voyage', testId: 'voyage-row', name: 'Rejs 2025', href: '/voyages/9' }
	])('stretches the $kind name link over its tile', async ({ testId, name, href }) => {
		await renderPage();
		const tile = await screen.findByTestId(testId);
		const links = within(tile).getAllByRole('link');
		expect(links).toHaveLength(1);
		const [link] = links;
		expect(link).toHaveAccessibleName(name);
		expect(link).toHaveAttribute('href', href);
		expect(tile).toHaveClass('relative', 'group');
		expect(link).toHaveClass('after:absolute', 'after:inset-0', '[overflow-wrap:anywhere]');
		expect(link.querySelector('a, button, input, textarea, select')).toBeNull();

		await fireEvent.click(link);
		expect(followed).toEqual([href]);
		expect(goto).not.toHaveBeenCalled();
	});

	it('signs up from the tile without opening the leg', async () => {
		fn(signUpForTrip).mockResolvedValue({
			trip_id: 50,
			result: 'created',
			enrollment: pending(50)
		});
		await renderPage();
		const tile = await screen.findByTestId('leg-row');
		const button = within(tile).getByRole('button', { name: 'Zapisz się: Jacht A' });
		const control = within(tile).getByTestId('leg-sign-up');
		expect(control.parentElement).toHaveClass('relative', 'z-10');
		expect(button.closest('a')).toBeNull();

		await fireEvent.click(button);

		await waitFor(() => expect(signUpForTrip).toHaveBeenCalledWith(50));
		expect(await within(tile).findByText('oczekujący')).toBeInTheDocument();
		expect(followed).toEqual([null]);
		expect(goto).not.toHaveBeenCalled();
	});

	it.each([
		{ state: 'closed sign-ups', over: { open: false }, text: 'Zapisy zamknięte' },
		{ state: 'a sign-up', over: { enrollment: pending(50) }, text: 'oczekujący' }
	])('lets the sign-up panel text with $state open the leg', async ({ over, text }) => {
		fn(listCruiseLegSignUps).mockResolvedValue([signUp(50, over)]);
		await renderPage();
		const tile = await screen.findByTestId('leg-row');
		const panel = within(tile).getByTestId('leg-sign-up').parentElement!;
		expect(panel).toHaveClass(
			'pointer-events-none',
			'[&_button]:pointer-events-auto',
			'[&_form]:pointer-events-auto'
		);
		expect(within(tile).getByText(text).closest('button, form')).toBeNull();
	});

	it('withdraws from the tile without opening the leg', async () => {
		fn(listCruiseLegSignUps).mockResolvedValue([signUp(50, { enrollment: pending(50) })]);
		fn(withdrawTripSignUp).mockResolvedValue(undefined);
		await renderPage();
		const tile = await screen.findByTestId('leg-row');

		await fireEvent.click(within(tile).getByRole('button', { name: 'Wycofaj zapis: Jacht A' }));

		await waitFor(() => expect(withdrawTripSignUp).toHaveBeenCalledWith(50));
		expect(
			await within(tile).findByRole('button', { name: 'Zapisz się: Jacht A' })
		).toBeInTheDocument();
		expect(followed).toEqual([null]);
		expect(goto).not.toHaveBeenCalled();
	});

	it('reaches each tile link by keyboard before its sign-up control', async () => {
		await renderPage();
		const tile = await screen.findByTestId('leg-row');
		const link = within(tile).getByRole('link', { name: 'Jacht A' });
		const button = within(tile).getByRole('button', { name: 'Zapisz się: Jacht A' });

		expect(link.tabIndex).toBe(0);
		link.focus();
		expect(link).toHaveFocus();
		expect(link).toHaveClass('focus-visible:after:ring-2');
		expect(link.compareDocumentPosition(button) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

		const voyage = within(screen.getByTestId('voyage-row')).getByRole('link', {
			name: 'Rejs 2025'
		});
		voyage.focus();
		expect(voyage).toHaveFocus();
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
				crew_member_id: 14,
				user_id: 4,
				person_name: 'Ola Nowak',
				email: 'ola@example.com',
				note: 'koja dziobowa'
			},
			{
				...pending(51),
				id: 2,
				crew_member_id: 15,
				user_id: undefined,
				person_name: 'Piotr Bezkonta',
				email: 'piotr@example.com',
				status: 'accepted'
			}
		]);
		fn(listCruiseEnrollments).mockResolvedValue([
			enrollmentRow,
			{ ...enrollmentRow, id: 33, user_name: 'Ewa Stara', trip_id: undefined, trip_name: undefined }
		]);
	});

	it('keeps the review link visible beside a long leg name', async () => {
		await renderPage();
		const a = await screen.findByRole('region', { name: 'Zapisy: Jacht A' });

		expect(within(a).getByRole('heading', { name: 'Jacht A' })).toHaveClass(
			'min-w-0',
			'[overflow-wrap:anywhere]'
		);
		expect(within(a).getByRole('link', { name: 'Zarządzaj zapisami' })).toHaveClass(
			'shrink-0',
			'whitespace-nowrap'
		);
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
		expect(within(a).queryByText('bez konta')).not.toBeInTheDocument();
		expect(within(b).getByText('Piotr Bezkonta')).toBeInTheDocument();
		expect(within(b).getByText('bez konta')).toBeInTheDocument();
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
		mockGet.mockResolvedValue(cruise());
		fn(listCruiseTrips).mockResolvedValue([
			{ ...tripRow(50, 'Jacht A', 'planned'), disembark_date: '2026-08-09' },
			{ ...tripRow(51, 'Jacht B', 'planned'), disembark_date: '2026-11-20' }
		]);
	});
	afterEach(() => {
		vi.useRealTimers();
	});

	it('flags a leg past its end and prompts an admin to complete it', async () => {
		mockAuth.isAdmin = true;
		await renderPage();
		await screen.findByRole('link', { name: 'Jacht A Ukryte' });

		const overdue = rowOf('Jacht A Ukryte');
		expect(within(overdue).getByRole('link', { name: 'Jacht A Ukryte' })).toHaveAttribute(
			'href',
			'/trips/50'
		);
		expect(within(overdue).getByText('Do rozliczenia')).toBeInTheDocument();
		expect(within(overdue).getByText('Zrealizuj →')).toBeInTheDocument();

		const ongoing = rowOf('Jacht B Ukryte');
		expect(within(ongoing).getByText('Nadchodzący')).toBeInTheDocument();
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

describe('cruise deletion during navigation', () => {
	it.each(['resolve', 'reject'] as const)(
		'ignores a late %s after leaving the cruise',
		async (result) => {
			mockAuth.isAdmin = true;
			mockGet.mockResolvedValue(cruise());
			vi.spyOn(window, 'confirm').mockReturnValue(true);
			let resolve!: () => void;
			let reject!: (error: Error) => void;
			fn(deleteCruise).mockReturnValue(
				new Promise<void>((yes, no) => {
					resolve = yes;
					reject = no;
				})
			);
			const page = await renderPage();
			await fireEvent.click(screen.getByRole('button', { name: 'Usuń' }));
			expect(deleteCruise).toHaveBeenCalledWith(7);
			mockGet.mockResolvedValue(cruise({ id: 9, name: 'Next cruise' }));
			await page.rerender({ data: await runLoad('9') });
			if (result === 'resolve') resolve();
			else reject(new Error('old deletion failed'));
			await new Promise((done) => setTimeout(done, 0));
			expect(goto).not.toHaveBeenCalled();
			expect(screen.queryByText('old deletion failed')).not.toBeInTheDocument();
			expect(screen.getByRole('heading', { name: 'Next cruise' })).toBeInTheDocument();
		}
	);
});

describe('cruise page review regressions', () => {
	it('combines page and admin failures in one top banner', async () => {
		mockAuth.isAdmin = true;
		mockGet.mockResolvedValue(cruise());
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(listCruiseTrips).mockRejectedValue(new Error('trips failed'));
		fn(listCruiseEnrollments).mockRejectedValue(new Error('enrollments failed'));
		await renderPage();
		await waitFor(() =>
			expect(screen.getByRole('alert')).toHaveTextContent('zgłoszenia na wydarzenie')
		);
		const alerts = screen.getAllByRole('alert');
		expect(alerts).toHaveLength(1);
		expect(alerts[0]).toHaveTextContent('nadchodzące etapy');
		expect(
			alerts[0].compareDocumentPosition(screen.getByRole('heading', { name: 'Sycylia 2026' })) &
				Node.DOCUMENT_POSITION_FOLLOWING
		).toBeTruthy();
	});
	it.each(['resolve', 'reject'] as const)('ignores deletion %s after unmount', async (result) => {
		mockAuth.isAdmin = true;
		mockGet.mockResolvedValue(cruise());
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		let resolve!: () => void;
		let reject!: (error: Error) => void;
		fn(deleteCruise).mockReturnValue(
			new Promise<void>((yes, no) => {
				resolve = yes;
				reject = no;
			})
		);
		const page = await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: 'Usuń' }));
		page.unmount();
		if (result === 'resolve') resolve();
		else reject(new Error('old deletion failed'));
		await new Promise((done) => setTimeout(done, 0));
		expect(goto).not.toHaveBeenCalled();
	});
});

describe('cruise page publication', () => {
	const at = '2026-03-01T10:00:00Z';
	const hiddenTrip = { ...tripRow(50, 'Jacht A', 'planned'), cruise_id: 7 };
	const publishedTrip = { ...tripRow(51, 'Jacht B', 'planned'), cruise_id: 7, published_at: at };
	const hiddenVoyage = { id: 9, cruise_id: 7, name: 'Rejs 2025', miles: 120, days: 7 };

	beforeEach(() => {
		mockAuth.isAdmin = true;
		mockAuth.user = { id: 1, role: 'admin', is_captain: false };
		mockGet.mockResolvedValue(cruise());
		fn(listCruiseTrips).mockResolvedValue([hiddenTrip, publishedTrip]);
		fn(listCruiseVoyages).mockResolvedValue([hiddenVoyage]);
		fn(publishCruise).mockImplementation(async () => cruise({ published_at: at }));
		fn(unpublishCruise).mockImplementation(async () => cruise());
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	const header = () => screen.getByTestId('cruise-header');
	const legRow = (name: string) =>
		screen.getAllByTestId('leg-row').find((r) => within(r).queryByText(name))!;
	const voyageRow = () => screen.getByTestId('voyage-row');

	it('shows an admin which cruise and legs are hidden', async () => {
		await renderPage();
		expect(within(header()).getByText('Ukryte')).toBeInTheDocument();
		expect(within(legRow('Jacht A')).getByText('Ukryte')).toBeInTheDocument();
		expect(within(legRow('Jacht B')).queryByText('Ukryte')).not.toBeInTheDocument();
		expect(within(voyageRow()).getByText('Ukryte')).toBeInTheDocument();
		expect(within(header()).getByRole('button', { name: 'Opublikuj' })).toBeInTheDocument();
		expect(within(header()).queryByRole('button', { name: 'Ukryj' })).not.toBeInTheDocument();
	});

	it.each([
		{ kind: 'leg', row: () => legRow('Jacht A'), href: '/trips/50' },
		{ kind: 'voyage', row: voyageRow, href: '/voyages/9' }
	])('opens the $kind from its hidden badge, which keeps its tooltip', async ({ row, href }) => {
		await renderPage();
		const badge = within(row()).getByTestId('hidden-badge');
		expect(badge).toHaveAttribute('title');
		expect(badge.closest('a')).toHaveAttribute('href', href);
		expect(badge.parentElement).toHaveClass('relative', 'z-10');
	});

	it('publishes the hidden legs too when the admin agrees', async () => {
		const ask = vi.spyOn(window, 'confirm').mockReturnValue(true);
		await renderPage();
		await fireEvent.click(within(header()).getByRole('button', { name: 'Opublikuj' }));

		expect(ask).toHaveBeenCalledWith('Opublikować też 2 ukryte etapy?');
		expect(publishCruise).toHaveBeenCalledWith(7, true);
		await waitFor(() => expect(screen.queryByText('Ukryte')).not.toBeInTheDocument());
		expect(within(header()).getByRole('button', { name: 'Ukryj' })).toBeInTheDocument();
	});

	it('publishes only the cruise when the admin declines the legs', async () => {
		vi.spyOn(window, 'confirm').mockReturnValue(false);
		await renderPage();
		await fireEvent.click(within(header()).getByRole('button', { name: 'Opublikuj' }));

		expect(publishCruise).toHaveBeenCalledWith(7, false);
		await waitFor(() => expect(within(header()).queryByText('Ukryte')).not.toBeInTheDocument());
		expect(within(legRow('Jacht A')).getByText('Ukryte')).toBeInTheDocument();
		expect(within(voyageRow()).getByText('Ukryte')).toBeInTheDocument();
	});

	it('asks nothing when every leg is published', async () => {
		const ask = vi.spyOn(window, 'confirm');
		fn(listCruiseTrips).mockResolvedValue([publishedTrip]);
		fn(listCruiseVoyages).mockResolvedValue([]);
		await renderPage();
		await fireEvent.click(within(header()).getByRole('button', { name: 'Opublikuj' }));

		expect(ask).not.toHaveBeenCalled();
		expect(publishCruise).toHaveBeenCalledWith(7, false);
		await waitFor(() => expect(screen.queryByText('Ukryte')).not.toBeInTheDocument());
	});

	it('hides a published cruise', async () => {
		mockGet.mockResolvedValue(cruise({ published_at: at }));
		await renderPage();
		expect(within(header()).queryByText('Ukryte')).not.toBeInTheDocument();
		await fireEvent.click(within(header()).getByRole('button', { name: 'Ukryj' }));

		expect(unpublishCruise).toHaveBeenCalledWith(7);
		expect(await within(header()).findByText('Ukryte')).toBeInTheDocument();
		expect(within(header()).getByRole('button', { name: 'Opublikuj' })).toBeInTheDocument();
	});

	it('keeps the badge and shows the error when publishing fails', async () => {
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(publishCruise).mockRejectedValue(new ApiError(500, 'failed to publish cruise'));
		await renderPage();
		await fireEvent.click(within(header()).getByRole('button', { name: 'Opublikuj' }));

		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Nie udało się opublikować wydarzenia: failed to publish cruise'
		);
		expect(within(header()).getByText('Ukryte')).toBeInTheDocument();
		expect(within(legRow('Jacht A')).getByText('Ukryte')).toBeInTheDocument();
		expect(within(header()).getByRole('button', { name: 'Opublikuj' })).toBeEnabled();
	});

	it('drops a publish reply for the cruise the page has left', async () => {
		vi.spyOn(window, 'confirm').mockReturnValue(false);
		let answer!: (c: Cruise) => void;
		fn(publishCruise).mockReturnValue(new Promise<Cruise>((yes) => (answer = yes)));
		mockGet.mockImplementation(async (id: number) =>
			cruise({ id, name: id === 7 ? 'Sycylia 2026' : 'Chorwacja 2026' })
		);
		const view = await renderPage();
		await fireEvent.click(within(header()).getByRole('button', { name: 'Opublikuj' }));
		await view.rerender({ data: await runLoad('8') });

		answer(cruise({ published_at: at }));
		await new Promise((done) => setTimeout(done, 0));
		expect(screen.getByRole('heading', { name: 'Chorwacja 2026' })).toBeInTheDocument();
		expect(within(header()).getByText('Ukryte')).toBeInTheDocument();
		expect(within(header()).getByRole('button', { name: 'Opublikuj' })).toBeEnabled();
	});

	it.each([
		['publishing opens', 'Opublikuj', false, true],
		['hiding closes', 'Ukryj', true, false]
	] as const)('reloads the sign-ups: %s them', async (_name, action, before, after) => {
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		if (action === 'Ukryj') mockGet.mockResolvedValue(cruise({ published_at: at }));
		fn(listCruiseLegSignUps)
			.mockResolvedValueOnce([{ trip_id: 50, open: before, accepted_count: 0 }])
			.mockResolvedValueOnce([{ trip_id: 50, open: after, accepted_count: 0 }]);
		await renderPage();
		const row = legRow('Jacht A');
		expect(within(row).queryByText('Zapisy zamknięte') === null).toBe(before);

		await fireEvent.click(within(header()).getByRole('button', { name: action }));

		await waitFor(() => expect(within(row).queryByText('Zapisy zamknięte') === null).toBe(after));
		expect(fn(listCruiseLegSignUps)).toHaveBeenLastCalledWith(7);
	});

	it('names the sign-ups when their reload fails after publishing', async () => {
		vi.spyOn(window, 'confirm').mockReturnValue(false);
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(listCruiseLegSignUps)
			.mockResolvedValueOnce([{ trip_id: 50, open: false, accepted_count: 0 }])
			.mockRejectedValueOnce(new Error('offline'));
		await renderPage();
		await fireEvent.click(within(header()).getByRole('button', { name: 'Opublikuj' }));

		expect(await screen.findByRole('alert')).toHaveTextContent('twoje zapisy na etapy');
		expect(within(header()).queryByText('Ukryte')).not.toBeInTheDocument();
	});

	it('shows the captain group the badges but no publish controls', async () => {
		mockAuth.isAdmin = false;
		mockAuth.user = { id: 3, role: 'member', is_captain: true, membership: 'czlonek' };
		await renderPage();
		expect(within(header()).getByText('Ukryte')).toBeInTheDocument();
		expect(within(legRow('Jacht A')).getByText('Ukryte')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Opublikuj' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Ukryj' })).not.toBeInTheDocument();
	});

	it('shows a member neither badges nor publish controls', async () => {
		mockAuth.isAdmin = false;
		mockAuth.user = { id: 2, role: 'member', is_captain: false, membership: 'czlonek' };
		await renderPage();
		expect(screen.queryByText('Ukryte')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Opublikuj' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Ukryj' })).not.toBeInTheDocument();
	});
});
