import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import type { Cruise } from '#lib/api/types.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: { params: { id: '7' } } }));
vi.mock('#lib/api/routes.js', () => ({
	getCruise: vi.fn(),
	getSeries: vi.fn(),
	deleteCruise: vi.fn(),
	listCruiseTrips: vi.fn(),
	listCruiseVoyages: vi.fn(),
	listCruiseEnrollments: vi.fn(),
	generateCruiseEnrollToken: vi.fn(),
	clearCruiseEnrollToken: vi.fn(),
	updateCruiseEnrollmentStatus: vi.fn(),
	assignCruiseEnrollmentTrip: vi.fn(),
	deleteCruiseEnrollment: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: false, user: { id: 2, role: 'member' } }
}));

import {
	getCruise,
	listCruiseTrips,
	listCruiseVoyages,
	listCruiseEnrollments
} from '#lib/api/routes.js';
import CruiseDetailPage from './+page.svelte';

const mockGet = getCruise as unknown as ReturnType<typeof vi.fn>;

function cruise(overrides: Partial<Cruise> = {}): Cruise {
	return {
		id: 7,
		name: 'Sycylia 2026',
		kind: 'sea',
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

beforeEach(() => {
	mockGet.mockReset();
	(listCruiseTrips as unknown as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([]);
	(listCruiseVoyages as unknown as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([]);
	(listCruiseEnrollments as unknown as ReturnType<typeof vi.fn>).mockReset().mockResolvedValue([]);
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
		render(CruiseDetailPage);

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
		render(CruiseDetailPage);

		expect(await screen.findByText('0+')).toBeInTheDocument();
	});

	it('renders as before when no logistics are set', async () => {
		mockGet.mockResolvedValue(cruise());
		render(CruiseDetailPage);

		expect(await screen.findByRole('heading', { name: 'Sycylia 2026' })).toBeInTheDocument();
		expect(screen.queryByText('Informacje dla załogi')).not.toBeInTheDocument();
		expect(screen.queryByTestId('cruise-logistics')).not.toBeInTheDocument();
		expect(screen.queryByRole('link', { name: 'Czat grupy' })).not.toBeInTheDocument();
		expect(screen.getByText('Zapisy')).toBeInTheDocument();
	});
});
