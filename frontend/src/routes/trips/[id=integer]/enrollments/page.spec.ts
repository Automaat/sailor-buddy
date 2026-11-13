import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/svelte';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: { params: { id: '50' } } }));
vi.mock('#lib/api/routes.js', () => ({
	getTrip: vi.fn(),
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
	listTripEnrollments,
	updateTripEnrollmentStatus,
	updateTripEnrollmentPaymentStatus
} from '#lib/api/routes.js';
import { makeTrip } from '#lib/test-utils.js';
import EnrollmentsPage from './+page.svelte';

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
		render(EnrollmentsPage);
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
		render(EnrollmentsPage);
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
		render(EnrollmentsPage);
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
		render(EnrollmentsPage);
		expect(await screen.findByLabelText('Płatność: Darek')).toHaveValue('paid');
		expect(counts().paid).toBe('Opłacone: 1');
	});

	it('reverts the control and shows an error when saving fails', async () => {
		fn(updateTripEnrollmentPaymentStatus).mockRejectedValue(new Error('403'));
		vi.spyOn(console, 'error').mockImplementation(() => {});
		render(EnrollmentsPage);
		const anna = await screen.findByLabelText('Płatność: Anna');

		await fireEvent.change(anna, { target: { value: 'unpaid' } });

		expect(await screen.findByRole('alert')).toHaveTextContent('Nie udało się zapisać statusu płatności');
		expect(anna).toHaveValue('paid');
		expect(counts().paid).toBe('Opłacone: 1');
	});

	it('hides the counts when there are no sign-ups', async () => {
		fn(listTripEnrollments).mockResolvedValue([]);
		render(EnrollmentsPage);
		expect(await screen.findByText('Brak zapisów.')).toBeInTheDocument();
		expect(screen.queryByTestId('payment-counts')).not.toBeInTheDocument();
	});
});
