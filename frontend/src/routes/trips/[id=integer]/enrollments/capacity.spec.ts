import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor, within } from '@testing-library/svelte';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/api/routes.js', () => ({
	getTrip: vi.fn(),
	getTripSignUp: vi.fn(),
	listTripEnrollments: vi.fn(),
	updateTripEnrollmentStatus: vi.fn(),
	deleteTripEnrollment: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));

import { ApiError } from '#lib/api/client.js';
import {
	getTrip,
	getTripSignUp,
	listTripEnrollments,
	updateTripEnrollmentStatus,
	deleteTripEnrollment
} from '#lib/api/routes.js';
import { makeTrip } from '#lib/test-utils.js';
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

function row(id: number, status: string, day: number) {
	return {
		id,
		trip_id: 50,
		user_id: 100 + id,
		user_name: `Osoba ${id}`,
		user_email: `osoba${id}@example.com`,
		status,
		created_at: `2026-01-0${day}T00:00:00Z`,
		updated_at: `2026-01-0${day}T00:00:00Z`
	};
}

// A two-place leg: both places accepted, two people waiting.
const fullLeg = () => [row(1, 'accepted', 1), row(2, 'accepted', 2), row(4, 'waitlisted', 4), row(3, 'waitlisted', 3)];

function card(name: string) {
	return screen.getByText(name).closest('div.rounded-xl') as HTMLElement;
}

beforeEach(() => {
	fn(getTrip).mockReset().mockResolvedValue(makeTrip({ id: 50, name: 'Jacht A', max_crew: 2 }));
	fn(getTripSignUp).mockReset().mockResolvedValue({ trip_id: 50, open: true, accepted_count: 2 });
	fn(listTripEnrollments).mockReset().mockResolvedValue(fullLeg());
	fn(updateTripEnrollmentStatus).mockReset().mockResolvedValue(undefined);
	fn(deleteTripEnrollment).mockReset().mockResolvedValue(undefined);
	vi.spyOn(window, 'confirm').mockReturnValue(true);
	vi.spyOn(console, 'error').mockImplementation(() => {});
});

describe('trip enrollments page capacity', () => {
	it('marks no one as next while the leg is full', async () => {
		await renderPage();
		expect(await screen.findByTestId('free-places')).toHaveTextContent('wolne 0 z 2');
		expect(screen.queryByTestId('next-waitlisted')).not.toBeInTheDocument();
	});

	it('explains a full leg when an accept is refused and keeps the status', async () => {
		fn(updateTripEnrollmentStatus).mockRejectedValue(new ApiError(409, 'leg is full: all 2 places are taken'));
		await renderPage();
		await screen.findByText('Osoba 3');

		await fireEvent.click(within(card('Osoba 3')).getByRole('button', { name: 'Akceptuj' }));

		expect(await screen.findByRole('alert')).toHaveTextContent('Etap jest pełny');
		expect(within(card('Osoba 3')).getByText('lista rezerwowa')).toBeInTheDocument();
	});

	it('shows a free place and marks the earliest waitlisted as next after a rejection', async () => {
		await renderPage();
		await screen.findByText('Osoba 1');

		await fireEvent.click(within(card('Osoba 1')).getByRole('button', { name: 'Odrzuć' }));

		await waitFor(() => expect(updateTripEnrollmentStatus).toHaveBeenCalledWith(50, 1, 'rejected'));
		expect(await screen.findByTestId('free-places')).toHaveTextContent('wolne 1 z 2');
		const next = screen.getByTestId('next-waitlisted');
		expect(card('Osoba 3')).toContainElement(next);
		expect(screen.getAllByTestId('next-waitlisted')).toHaveLength(1);
		expect(updateTripEnrollmentStatus).not.toHaveBeenCalledWith(50, 3, 'accepted');
	});

	it('marks the next waitlisted after an accepted sign-up is deleted', async () => {
		await renderPage();
		await screen.findByText('Osoba 2');

		await fireEvent.click(within(card('Osoba 2')).getByRole('button', { name: 'Usuń' }));

		await waitFor(() => expect(deleteTripEnrollment).toHaveBeenCalledWith(50, 2));
		expect(within(card('Osoba 3')).getByTestId('next-waitlisted')).toBeInTheDocument();
	});

	it('counts cruise-level accepted crew toward the places', async () => {
		fn(listTripEnrollments).mockResolvedValue([row(1, 'accepted', 1), row(3, 'waitlisted', 3)]);
		await renderPage();
		expect(await screen.findByTestId('free-places')).toHaveTextContent('wolne 0 z 2');
		expect(screen.queryByTestId('next-waitlisted')).not.toBeInTheDocument();
	});
});
