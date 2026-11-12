import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';

const h = vi.hoisted(() => ({ membership: 'czlonek' as 'czlonek' | 'sympatyk' | 'gosc' }));

vi.mock('$app/state', () => ({ page: { params: { id: '7' } } }));
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: {
		isAdmin: false,
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

import { getCruise } from '#lib/api/routes.js';
import CruisePage from './+page.svelte';

const getCruiseMock = getCruise as unknown as ReturnType<typeof vi.fn>;

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
	getCruiseMock.mockReset();
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
