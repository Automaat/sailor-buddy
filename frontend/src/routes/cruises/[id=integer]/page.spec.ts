import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

const h = vi.hoisted(() => ({
	membership: 'czlonek' as 'czlonek' | 'sympatyk' | 'gosc',
	admin: false
}));

vi.mock('$app/state', () => ({ page: { params: { id: '7' } } }));
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: {
		get isAdmin() {
			return h.admin;
		},
		get user() {
			return { membership: h.membership };
		}
	}
}));
vi.mock('#lib/api/routes.js', () => ({
	getCruise: vi.fn(),
	getSeries: vi.fn(),
	deleteCruise: vi.fn(),
	listCruiseTrips: vi.fn().mockResolvedValue([]),
	listCruiseVoyages: vi.fn().mockResolvedValue([]),
	listCruiseEnrollments: vi.fn().mockResolvedValue([]),
	generateCruiseEnrollToken: vi.fn(),
	clearCruiseEnrollToken: vi.fn(),
	updateCruiseEnrollmentStatus: vi.fn(),
	assignCruiseEnrollmentTrip: vi.fn(),
	deleteCruiseEnrollment: vi.fn()
}));

import {
	getCruise,
	listCruiseTrips,
	listCruiseEnrollments,
	generateCruiseEnrollToken,
	updateCruiseEnrollmentStatus,
	assignCruiseEnrollmentTrip
} from '#lib/api/routes.js';
import CruisePage from './+page.svelte';

type Mock = ReturnType<typeof vi.fn>;
const getCruiseMock = getCruise as unknown as Mock;
const listTripsMock = listCruiseTrips as unknown as Mock;
const listEnrollmentsMock = listCruiseEnrollments as unknown as Mock;

function cruise(overrides: Record<string, unknown> = {}) {
	return {
		id: 7,
		name: 'WRM 2025',
		kind: 'inland',
		currency: 'PLN',
		included: [],
		not_included: [],
		created_at: '2025-01-01T00:00:00Z',
		updated_at: '2025-01-01T00:00:00Z',
		...overrides
	};
}

beforeEach(() => {
	h.membership = 'czlonek';
	h.admin = false;
	getCruiseMock.mockReset();
	listTripsMock.mockReset().mockResolvedValue([]);
	listEnrollmentsMock.mockReset().mockResolvedValue([]);
});

describe('cruise page — price', () => {
	it.each([
		['czlonek', '820 PLN'],
		['sympatyk', '820 PLN'],
		['gosc', '870 PLN']
	] as const)('shows %s their price and the included lists', async (membership, want) => {
		h.membership = membership;
		getCruiseMock.mockResolvedValue(
			cruise({
				cost_per_person: 820,
				cost_per_person_non_member: 870,
				included: ['jacht', 'paliwo'],
				not_included: ['dojazd']
			})
		);
		render(CruisePage);
		expect(await screen.findByTestId('your-price')).toHaveTextContent(`Twoja cena: ${want}`);
		expect(screen.getByTestId('price-included')).toHaveTextContent('paliwo');
		expect(screen.getByTestId('price-not-included')).toHaveTextContent('dojazd');
	});

	it('hides the price card for an unpriced cruise', async () => {
		getCruiseMock.mockResolvedValue(cruise({ cost_per_person: 0 }));
		render(CruisePage);
		expect(await screen.findByText('WRM 2025')).toBeInTheDocument();
		expect(screen.queryByRole('heading', { name: 'Cena' })).not.toBeInTheDocument();
	});
});

describe('cruise page — admin enrollments', () => {
	const pending = { id: 3, user_name: 'Ola', user_email: 'ola@example.dev', status: 'pending' };

	beforeEach(() => {
		h.admin = true;
		getCruiseMock.mockResolvedValue(cruise());
		listTripsMock.mockResolvedValue([
			{ id: 11, name: 'Ilmatar', status: 'planned', created_at: '', updated_at: '' }
		]);
	});

	it('opens enrollment and shows the share link button', async () => {
		(generateCruiseEnrollToken as unknown as Mock).mockResolvedValue({ token: 'abc' });
		render(CruisePage);
		await fireEvent.click(await screen.findByRole('button', { name: 'Włącz zapisy' }));
		expect(await screen.findByRole('button', { name: 'Kopiuj link' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Wyłącz zapisy' })).toBeInTheDocument();
	});

	it('accepts a pending enrollment and assigns it to a yacht', async () => {
		listEnrollmentsMock.mockResolvedValue([pending]);
		(updateCruiseEnrollmentStatus as unknown as Mock).mockResolvedValue(undefined);
		(assignCruiseEnrollmentTrip as unknown as Mock).mockResolvedValue(undefined);
		render(CruisePage);
		expect(await screen.findByText('Ola')).toBeInTheDocument();
		expect(screen.getByText('0 / 1 zaakceptowanych')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Akceptuj' }));
		await waitFor(() => expect(updateCruiseEnrollmentStatus).toHaveBeenCalledWith(7, 3, 'accepted'));
		expect(await screen.findByText('1 / 1 zaakceptowanych')).toBeInTheDocument();

		await fireEvent.change(screen.getByRole('combobox'), { target: { value: '11' } });
		await waitFor(() => expect(assignCruiseEnrollmentTrip).toHaveBeenCalledWith(7, 3, 11));
	});

	it('lists the planned yachts with an add link', async () => {
		render(CruisePage);
		expect(await screen.findByText('Planowane rejsy (1)')).toBeInTheDocument();
		expect(screen.getByRole('link', { name: '+ Dodaj jacht' })).toHaveAttribute('href', '/trips/new?cruise_id=7');
	});
});
