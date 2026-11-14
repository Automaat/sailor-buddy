import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/svelte';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
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
	getTripSignUp: vi.fn(),
	signUpForTrip: vi.fn(),
	updateTripSignUpNote: vi.fn(),
	withdrawTripSignUp: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: false, user: { id: 2, role: 'member', membership: 'czlonek' } }
}));

import {
	getTrip,
	listTripCrew,
	listAssignableCrew,
	getCruise,
	getTripSignUp,
	signUpForTrip,
	updateTripSignUpNote,
	withdrawTripSignUp
} from '#lib/api/routes.js';
import { makeTrip } from '#lib/test-utils.js';
import TripPage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

function runLoad(id = '50') {
	return load({ params: { id } } as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

// renderPage runs the route's load function and renders the page with its
// data, the way SvelteKit does on navigation.
async function renderPage() {
	return render(TripPage, { props: { data: await runLoad() } });
}

const fn = (f: unknown) => f as ReturnType<typeof vi.fn>;

function enrollment(overrides: Record<string, unknown> = {}) {
	return {
		id: 9,
		trip_id: 50,
		user_id: 2,
		status: 'pending',
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

beforeEach(() => {
	fn(getTrip)
		.mockReset()
		.mockResolvedValue(makeTrip({ id: 50, cruise_id: 7, name: 'Jacht A', max_crew: 8 }));
	fn(listTripCrew).mockReset().mockResolvedValue([]);
	fn(listAssignableCrew).mockReset().mockResolvedValue([]);
	fn(getCruise).mockReset().mockResolvedValue({ id: 7, name: 'WSEX 2026' });
	fn(getTripSignUp).mockReset().mockResolvedValue({ trip_id: 50, open: true, accepted_count: 3 });
	for (const f of [signUpForTrip, updateTripSignUpNote, withdrawTripSignUp]) fn(f).mockReset();
});

describe('trip page sign-up', () => {
	it('shows free places and a one-click sign-up button', async () => {
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');
		expect(within(card).getByTestId('free-places')).toHaveTextContent('wolne 5 z 8');
		expect(within(card).getByRole('button', { name: 'Zapisz się: Jacht A' })).toBeEnabled();
		expect(within(card).queryByRole('textbox')).not.toBeInTheDocument();
	});

	it('signs up and shows the pending status without a reload', async () => {
		fn(signUpForTrip).mockResolvedValue({
			trip_id: 50,
			result: 'created',
			enrollment: enrollment()
		});
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');

		await fireEvent.click(within(card).getByRole('button', { name: 'Zapisz się: Jacht A' }));

		await waitFor(() => expect(signUpForTrip).toHaveBeenCalledWith(50));
		expect(await within(card).findByText('oczekujący')).toBeInTheDocument();
		expect(
			within(card).getByRole('button', { name: 'Wycofaj zapis: Jacht A' })
		).toBeInTheDocument();
		expect(getTripSignUp).toHaveBeenCalledTimes(1);
	});

	it.each([
		['accepted', 'zaakceptowany'],
		['rejected', 'odrzucony']
	])('shows a reviewed (%s) sign-up without withdraw', async (status, label) => {
		fn(getTripSignUp).mockResolvedValue({
			trip_id: 50,
			open: true,
			accepted_count: 3,
			enrollment: enrollment({ status })
		});
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');
		expect(within(card).getByText(label)).toBeInTheDocument();
		expect(within(card).queryByRole('button', { name: /Wycofaj/ })).not.toBeInTheDocument();
	});

	it('shows a waitlisted sign-up with its position and lets the member leave the list', async () => {
		fn(getTripSignUp).mockResolvedValue({
			trip_id: 50,
			open: true,
			accepted_count: 8,
			enrollment: enrollment({ status: 'waitlisted' }),
			waitlist_position: 2
		});
		fn(withdrawTripSignUp).mockResolvedValue(undefined);
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');
		expect(within(card).getByTestId('sign-up-status')).toHaveTextContent('lista rezerwowa (2.)');
		expect(within(card).getByTestId('free-places')).toHaveTextContent('wolne 0 z 8');

		await fireEvent.click(within(card).getByRole('button', { name: 'Wycofaj zapis: Jacht A' }));

		await waitFor(() => expect(withdrawTripSignUp).toHaveBeenCalledWith(50));
		expect(
			await within(card).findByRole('button', { name: 'Zapisz się: Jacht A' })
		).toBeInTheDocument();
	});

	it('signs up on a full leg and shows the waiting-list position at once', async () => {
		fn(getTripSignUp).mockResolvedValue({ trip_id: 50, open: true, accepted_count: 8 });
		fn(signUpForTrip).mockResolvedValue({
			trip_id: 50,
			result: 'created',
			enrollment: enrollment({ status: 'waitlisted' }),
			waitlist_position: 3
		});
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');

		await fireEvent.click(within(card).getByRole('button', { name: 'Zapisz się: Jacht A' }));

		expect(await within(card).findByTestId('sign-up-status')).toHaveTextContent(
			'lista rezerwowa (3.)'
		);
		expect(getTripSignUp).toHaveBeenCalledTimes(1);
	});

	it.each([
		['unpaid', 'Brak wpłaty'],
		['deposit', 'Zaliczka wpłacona'],
		['paid', 'Opłacone']
	])('shows the %s payment status next to the sign-up status', async (payment, label) => {
		fn(getTripSignUp).mockResolvedValue({
			trip_id: 50,
			open: true,
			accepted_count: 3,
			enrollment: enrollment({ payment_status: payment })
		});
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');
		expect(within(card).getByTestId('sign-up-status')).toHaveTextContent('oczekujący');
		expect(within(card).getByTestId('sign-up-payment')).toHaveTextContent(label);
	});

	it.each(['deposit', 'paid'])(
		'hides withdraw on a pending sign-up with a %s payment',
		async (payment) => {
			fn(getTripSignUp).mockResolvedValue({
				trip_id: 50,
				open: true,
				accepted_count: 3,
				enrollment: enrollment({ payment_status: payment })
			});
			await renderPage();
			const card = await screen.findByTestId('my-sign-up');
			expect(within(card).getByTestId('sign-up-status')).toHaveTextContent('oczekujący');
			expect(within(card).queryByRole('button', { name: /Wycofaj/ })).not.toBeInTheDocument();
		}
	);

	it('shows no payment status for a cruise-level enrollment', async () => {
		fn(getTripSignUp).mockResolvedValue({
			trip_id: 50,
			open: true,
			accepted_count: 3,
			enrollment: enrollment({ cruise_id: 7 })
		});
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');
		expect(within(card).getByTestId('sign-up-status')).toBeInTheDocument();
		expect(within(card).queryByTestId('sign-up-payment')).not.toBeInTheDocument();
	});

	it('withdraws a pending sign-up', async () => {
		fn(getTripSignUp).mockResolvedValue({
			trip_id: 50,
			open: true,
			accepted_count: 3,
			enrollment: enrollment()
		});
		fn(withdrawTripSignUp).mockResolvedValue(undefined);
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');

		await fireEvent.click(within(card).getByRole('button', { name: 'Wycofaj zapis: Jacht A' }));

		await waitFor(() => expect(withdrawTripSignUp).toHaveBeenCalledWith(50));
		expect(
			await within(card).findByRole('button', { name: 'Zapisz się: Jacht A' })
		).toBeInTheDocument();
	});

	it('edits the note after signing up', async () => {
		fn(getTripSignUp).mockResolvedValue({
			trip_id: 50,
			open: true,
			accepted_count: 3,
			enrollment: enrollment({ note: 'stara' })
		});
		fn(updateTripSignUpNote).mockResolvedValue(enrollment({ note: 'nowa' }));
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');
		expect(within(card).getByTestId('sign-up-note')).toHaveTextContent('stara');

		await fireEvent.click(within(card).getByRole('button', { name: 'Edytuj notatkę' }));
		const box = within(card).getByLabelText('Notatka dla organizatora');
		expect(box).toHaveValue('stara');
		await fireEvent.input(box, { target: { value: 'nowa' } });
		await fireEvent.click(within(card).getByRole('button', { name: 'Zapisz notatkę' }));

		await waitFor(() => expect(updateTripSignUpNote).toHaveBeenCalledWith(50, 'nowa'));
		expect(await within(card).findByTestId('sign-up-note')).toHaveTextContent('nowa');
	});

	it('shows closed sign-ups instead of the button', async () => {
		fn(getTripSignUp).mockResolvedValue({ trip_id: 50, open: false, accepted_count: 0 });
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');
		expect(within(card).getByText('Zapisy zamknięte')).toBeInTheDocument();
		expect(within(card).queryByRole('button', { name: /Zapisz się/ })).not.toBeInTheDocument();
	});

	it('shows the sign-up deadline', async () => {
		fn(getTrip).mockResolvedValue(
			makeTrip({
				id: 50,
				cruise_id: 7,
				name: 'Jacht A',
				max_crew: 8,
				sign_up_deadline: '2026-06-01'
			})
		);
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');
		expect(within(card).getByTestId('sign-up-deadline')).toHaveTextContent('Zapisy do 1 cze 2026');
		expect(within(card).getByRole('button', { name: 'Zapisz się: Jacht A' })).toBeEnabled();
	});

	it('shows Zapisy zamknięte once the deadline has passed', async () => {
		fn(getTrip).mockResolvedValue(
			makeTrip({
				id: 50,
				cruise_id: 7,
				name: 'Jacht A',
				max_crew: 8,
				sign_up_deadline: '2026-06-01'
			})
		);
		fn(getTripSignUp).mockResolvedValue({ trip_id: 50, open: false, accepted_count: 0 });
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');
		expect(within(card).getByText('Zapisy zamknięte')).toBeInTheDocument();
		expect(within(card).getByTestId('sign-up-deadline')).toHaveTextContent('Zapisy do 1 cze 2026');
		expect(within(card).queryByRole('button', { name: /Zapisz się/ })).not.toBeInTheDocument();
	});

	it('omits the deadline when the leg has none', async () => {
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');
		expect(within(card).queryByTestId('sign-up-deadline')).not.toBeInTheDocument();
	});

	it('shows a cancelled leg without free places or a button', async () => {
		fn(getTrip).mockResolvedValue(
			makeTrip({ id: 50, cruise_id: 7, name: 'Jacht A', max_crew: 8, status: 'cancelled' })
		);
		fn(getTripSignUp).mockResolvedValue({ trip_id: 50, open: false, accepted_count: 0 });
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');
		expect(within(card).getByText('Etap odwołany')).toBeInTheDocument();
		expect(within(card).queryByTestId('free-places')).not.toBeInTheDocument();
	});

	it('omits free places when the leg has no crew limit', async () => {
		fn(getTrip).mockResolvedValue(makeTrip({ id: 50, cruise_id: 7, name: 'Jacht A' }));
		await renderPage();
		const card = await screen.findByTestId('my-sign-up');
		expect(within(card).queryByTestId('free-places')).not.toBeInTheDocument();
	});

	it('hides the sign-up card when the sign-up state cannot load', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(getTripSignUp).mockRejectedValue(new Error('boom'));
		await renderPage();
		await screen.findByRole('heading', { name: 'Jacht A' });
		expect(screen.queryByTestId('my-sign-up')).not.toBeInTheDocument();
		expect(screen.getByText(/Nie udało się wczytać: zapis na etap\./)).toBeInTheDocument();
	});
});
