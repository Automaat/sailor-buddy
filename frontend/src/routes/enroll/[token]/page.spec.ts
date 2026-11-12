import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import { makeTrip } from '#lib/test-utils.js';
import type { EnrollPageData } from '#lib/api/types.js';

vi.mock('$app/state', () => ({ page: { params: { token: 'tok' } } }));
vi.mock('#lib/api/routes.js', () => ({ resolveEnroll: vi.fn(), enroll: vi.fn() }));
vi.mock('#lib/stores/auth.svelte.js', () => ({ auth: { isAuthenticated: true } }));

import { resolveEnroll } from '#lib/api/routes.js';
import EnrollPage from './+page.svelte';

const resolveMock = resolveEnroll as unknown as ReturnType<typeof vi.fn>;

function tripData(maxCrew: number | undefined, accepted: number): EnrollPageData {
	return {
		kind: 'trip',
		trip: { id: 1, name: 'Rejs Bałtyk', max_crew: maxCrew },
		enrolled: false,
		accepted_count: accepted,
		total_count: accepted + 2
	};
}

beforeEach(() => {
	resolveMock.mockReset();
});

describe('enroll page free places', () => {
	it('shows the crew limit minus accepted enrollments for a trip', async () => {
		resolveMock.mockResolvedValue(tripData(10, 3));
		render(EnrollPage);
		const free = await screen.findByTestId('free-places');
		expect(free).toHaveTextContent('Wolne miejsca: 7');
		expect(resolveMock).toHaveBeenCalledWith('tok');
	});

	it('shows zero and keeps enrollment open when the trip is full', async () => {
		resolveMock.mockResolvedValue(tripData(4, 6));
		render(EnrollPage);
		expect(await screen.findByTestId('free-places')).toHaveTextContent('Wolne miejsca: 0');
		expect(screen.getByRole('button', { name: 'Zapisz się' })).toBeEnabled();
	});

	it.each([
		['missing', undefined],
		['zero, the forms value for no limit', 0]
	])('shows nothing for a trip whose crew limit is %s', async (_label, maxCrew) => {
		resolveMock.mockResolvedValue(tripData(maxCrew, 3));
		render(EnrollPage);
		await screen.findByText('Rejs Bałtyk');
		expect(screen.queryByTestId('free-places')).not.toBeInTheDocument();
		expect(screen.queryByText(/Wolne miejsca/i)).not.toBeInTheDocument();
	});

	it('counts cruise enrollments assigned to the trip when the API sends them', async () => {
		resolveMock.mockResolvedValue({
			...tripData(10, 2),
			trip_accepted: [{ trip_id: 1, accepted_count: 5 }]
		});
		render(EnrollPage);
		expect(await screen.findByTestId('free-places')).toHaveTextContent('Wolne miejsca: 5');
	});

	it('shows free places per cruise trip from its accepted count', async () => {
		resolveMock.mockResolvedValue({
			kind: 'cruise',
			cruise: { id: 9, name: 'Wielki Rejs Mazurski' },
			trips: [
				makeTrip({ id: 1, name: 'Ilmatar', max_crew: 10 }),
				makeTrip({ id: 2, name: 'Kliper', max_crew: 8 }),
				makeTrip({ id: 3, name: 'Bez limitu', max_crew: undefined }),
				makeTrip({ id: 4, name: 'Zero', max_crew: 0 })
			],
			trip_accepted: [
				{ trip_id: 1, accepted_count: 4 },
				{ trip_id: 3, accepted_count: 2 }
			],
			enrolled: false,
			accepted_count: 6,
			total_count: 9
		} satisfies EnrollPageData);
		render(EnrollPage);

		const ilmatar = (await screen.findByText('Ilmatar')).parentElement;
		expect(ilmatar).toHaveTextContent('wolne miejsca: 6');
		const kliper = screen.getByText('Kliper').parentElement;
		expect(kliper).toHaveTextContent('wolne miejsca: 8');
		const unlimited = screen.getByText('Bez limitu').parentElement;
		expect(unlimited).not.toHaveTextContent(/wolne miejsca/i);
		expect(screen.getByText('Zero').parentElement).not.toHaveTextContent(/wolne miejsca/i);
		expect(screen.getAllByTestId('free-places')).toHaveLength(2);
	});
});
