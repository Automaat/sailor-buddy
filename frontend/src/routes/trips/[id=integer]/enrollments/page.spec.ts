import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/svelte';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/api/routes.js', () => ({
	getTrip: vi.fn(),
	getTripSignUp: vi.fn(),
	listTripEnrollments: vi.fn(),
	updateTripEnrollmentStatus: vi.fn(),
	updateTripEnrollmentPaymentStatus: vi.fn(),
	deleteTripEnrollment: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));

import {
	getTrip,
	getTripSignUp,
	listTripEnrollments,
	updateTripEnrollmentStatus,
	updateTripEnrollmentPaymentStatus
} from '#lib/api/routes.js';
import { makeTrip } from '#lib/test-utils.js';
import { ApiError } from '#lib/api/client.js';
import EnrollmentsPage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

function runLoad(id = '50') {
	return load({ params: { id } } as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

// renderPage runs the route's load function and renders the page with its
// data, the way SvelteKit does on navigation.
async function renderPage() {
	return render(EnrollmentsPage, { props: { data: await runLoad() } });
}

const fn = (f: unknown) => f as ReturnType<typeof vi.fn>;

function row(id: number, name: string, overrides: Record<string, unknown> = {}) {
	return {
		id,
		trip_id: 50,
		user_id: id,
		user_name: name,
		user_email: `${name.toLowerCase()}@example.com`,
		status: 'pending',
		payment_status: 'unpaid',
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

beforeEach(() => {
	fn(getTrip).mockReset().mockResolvedValue(makeTrip({ id: 50, name: 'Jacht A', max_crew: 8 }));
	fn(getTripSignUp).mockReset().mockResolvedValue({ trip_id: 50, open: true, accepted_count: 1 });
	fn(listTripEnrollments)
		.mockReset()
		.mockResolvedValue([
			row(1, 'Anna', { payment_status: 'paid', status: 'accepted' }),
			row(2, 'Bartek', { payment_status: 'deposit' }),
			row(3, 'Celina', { payment_status: 'deposit' }),
			row(4, 'Darek')
		]);
	fn(updateTripEnrollmentStatus).mockReset();
	fn(updateTripEnrollmentPaymentStatus).mockReset();
});

function counts() {
	const box = screen.getByTestId('payment-counts');
	return {
		unpaid: within(box).getByTestId('payment-count-unpaid').textContent,
		deposit: within(box).getByTestId('payment-count-deposit').textContent,
		paid: within(box).getByTestId('payment-count-paid').textContent
	};
}

describe('trip enrollments page payment status', () => {
	it('shows each sign-up payment status and the counts per status', async () => {
		await renderPage();
		const anna = await screen.findByLabelText('Płatność: Anna');
		expect(anna).toHaveValue('paid');
		expect(screen.getByLabelText('Płatność: Bartek')).toHaveValue('deposit');
		expect(screen.getByLabelText('Płatność: Darek')).toHaveValue('unpaid');
		expect(counts()).toEqual({
			unpaid: 'Brak wpłaty: 1',
			deposit: 'Zaliczka wpłacona: 2',
			paid: 'Opłacone: 1'
		});
	});

	it('sets the payment status and updates the counts without touching acceptance', async () => {
		fn(updateTripEnrollmentPaymentStatus).mockResolvedValue(row(4, 'Darek', { payment_status: 'deposit' }));
		await renderPage();
		const darek = await screen.findByLabelText('Płatność: Darek');

		await fireEvent.change(darek, { target: { value: 'deposit' } });

		await waitFor(() => expect(updateTripEnrollmentPaymentStatus).toHaveBeenCalledWith(50, 4, 'deposit'));
		await waitFor(() => expect(counts().deposit).toBe('Zaliczka wpłacona: 3'));
		expect(counts().unpaid).toBe('Brak wpłaty: 0');
		expect(updateTripEnrollmentStatus).not.toHaveBeenCalled();
		expect(screen.getAllByText('oczekujący')).toHaveLength(3);
	});

	it('disables the control while its change is saving', async () => {
		let resolve: (v: unknown) => void = () => {};
		fn(updateTripEnrollmentPaymentStatus).mockReturnValue(new Promise((r) => (resolve = r)));
		await renderPage();
		const darek = await screen.findByLabelText('Płatność: Darek');

		await fireEvent.change(darek, { target: { value: 'paid' } });

		await waitFor(() => expect(darek).toBeDisabled());
		expect(screen.getByLabelText('Płatność: Anna')).toBeEnabled();
		resolve(row(4, 'Darek', { payment_status: 'paid' }));
		await waitFor(() => expect(darek).toBeEnabled());
		expect(counts().paid).toBe('Opłacone: 2');
	});

	it('shows the saved payment status after a reload', async () => {
		fn(listTripEnrollments).mockResolvedValue([row(4, 'Darek', { payment_status: 'paid' })]);
		await renderPage();
		expect(await screen.findByLabelText('Płatność: Darek')).toHaveValue('paid');
		expect(counts().paid).toBe('Opłacone: 1');
	});

	it('reverts the control and shows an error when saving fails', async () => {
		fn(updateTripEnrollmentPaymentStatus).mockRejectedValue(new Error('403'));
		vi.spyOn(console, 'error').mockImplementation(() => {});
		await renderPage();
		const anna = await screen.findByLabelText('Płatność: Anna');

		await fireEvent.change(anna, { target: { value: 'unpaid' } });

		expect(await screen.findByRole('alert')).toHaveTextContent('Nie udało się zapisać statusu płatności');
		expect(anna).toHaveValue('paid');
		expect(counts().paid).toBe('Opłacone: 1');
	});

	it('hides the counts when there are no sign-ups', async () => {
		fn(listTripEnrollments).mockResolvedValue([]);
		await renderPage();
		expect(await screen.findByText('Brak zapisów.')).toBeInTheDocument();
		expect(screen.queryByTestId('payment-counts')).not.toBeInTheDocument();
	});
});

describe('trip enrollments page load errors', () => {
	it('flags the accepted count when the leg sign-up state fails, and still lists sign-ups', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(getTripSignUp).mockRejectedValue(new Error('500'));
		await renderPage();

		expect(await screen.findByText(/Nie udało się wczytać: liczba zajętych miejsc\./)).toBeInTheDocument();
		expect(screen.getByLabelText('Płatność: Anna')).toBeInTheDocument();
	});

	it('fails the load instead of showing an empty list when the sign-ups fail', async () => {
		const boom = new Error('500');
		fn(listTripEnrollments).mockRejectedValue(boom);

		await expect(runLoad()).rejects.toBe(boom);
	});

	it('shows no load banner when every section loads', async () => {
		await renderPage();

		await screen.findByLabelText('Płatność: Anna');
		expect(screen.queryByText(/Nie udało się wczytać/)).not.toBeInTheDocument();
	});
});

describe('trip enrollments page load', () => {
	it('throws a 404 with the trip copy for +error.svelte', async () => {
		fn(getTrip).mockRejectedValue(new ApiError(404, 'not found'));

		await expect(runLoad()).rejects.toMatchObject({
			status: 404,
			body: { title: 'Nie znaleziono rejsu', backHref: '/cruises' }
		});
	});

	it('sends a member the API refuses back to the trip', async () => {
		fn(listTripEnrollments).mockRejectedValue(new ApiError(403, 'forbidden'));

		await expect(runLoad()).rejects.toMatchObject({ status: 307, location: '/trips/50' });
	});

	it('loads the trip named by the URL', async () => {
		await runLoad('9');

		expect(getTrip).toHaveBeenCalledWith(9);
		expect(listTripEnrollments).toHaveBeenCalledWith(9);
		expect(getTripSignUp).toHaveBeenCalledWith(9);
	});
});

describe('trip enrollments page same-route navigation', () => {
	beforeEach(() => {
		fn(getTrip).mockImplementation(async (id: number) =>
			makeTrip({ id, name: id === 50 ? 'Jacht A' : 'Jacht B', max_crew: 8 })
		);
		fn(listTripEnrollments).mockImplementation(async (id: number) =>
			id === 50 ? [row(1, 'Anna')] : [row(7, 'Zenon', { trip_id: 51 })]
		);
	});

	it('swaps to the second trip and clears the first trip alerts', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		fn(updateTripEnrollmentPaymentStatus).mockRejectedValue(new Error('500'));
		const view = await renderPage();
		const select = await screen.findByLabelText('Płatność: Anna');
		await fireEvent.change(select, { target: { value: 'paid' } });
		expect(await screen.findByText('Nie udało się zapisać statusu płatności')).toBeInTheDocument();

		await view.rerender({ data: await runLoad('51') });

		expect(screen.getByText(/Jacht B/)).toBeInTheDocument();
		expect(screen.getByLabelText('Płatność: Zenon')).toBeInTheDocument();
		expect(screen.queryByLabelText('Płatność: Anna')).not.toBeInTheDocument();
		await waitFor(() =>
			expect(screen.queryByText('Nie udało się zapisać statusu płatności')).not.toBeInTheDocument()
		);
	});

	it('shows the second trip payment status when a link moves away mid-save', async () => {
		let answer: (v: unknown) => void = () => {};
		fn(updateTripEnrollmentPaymentStatus).mockReturnValue(new Promise((r) => (answer = r)));
		const view = await renderPage();
		const anna = await screen.findByLabelText('Płatność: Anna');
		await fireEvent.change(anna, { target: { value: 'paid' } });
		await waitFor(() => expect(anna).toBeDisabled());

		await view.rerender({ data: await runLoad('51') });

		const zenon = screen.getByLabelText('Płatność: Zenon');
		expect(zenon).toHaveValue('unpaid');
		expect(zenon).toBeEnabled();
		answer(row(1, 'Anna', { payment_status: 'paid' }));
		await waitFor(() => expect(updateTripEnrollmentPaymentStatus).toHaveBeenCalledTimes(1));
		await Promise.resolve();
		expect(screen.getByLabelText('Płatność: Zenon')).toHaveValue('unpaid');
		expect(counts().unpaid).toBe('Brak wpłaty: 1');
	});

	it('drops a status change that answers after a link moved to another trip', async () => {
		let answer: () => void = () => {};
		fn(updateTripEnrollmentStatus).mockReturnValue(new Promise<void>((r) => (answer = r)));
		fn(listTripEnrollments).mockImplementation(async (id: number) =>
			id === 50 ? [row(1, 'Anna')] : [row(1, 'Anna', { trip_id: 51 })]
		);
		const view = await renderPage();
		await screen.findByLabelText('Płatność: Anna');
		await fireEvent.click(screen.getByRole('button', { name: 'Akceptuj' }));

		await view.rerender({ data: await runLoad('51') });
		answer();
		await Promise.resolve();
		await Promise.resolve();

		expect(screen.getByText(/Jacht B/)).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Akceptuj' })).toBeInTheDocument();
	});
});
