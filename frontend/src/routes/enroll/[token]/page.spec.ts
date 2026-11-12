import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

const h = vi.hoisted(() => ({ authenticated: true }));

vi.mock('$app/state', () => ({ page: { params: { token: 'tok' } } }));
vi.mock('#lib/api/routes.js', () => ({
	resolveEnroll: vi.fn(),
	enroll: vi.fn(),
	enrollLegs: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: {
		get isAuthenticated() {
			return h.authenticated;
		}
	}
}));

import { resolveEnroll, enroll, enrollLegs } from '#lib/api/routes.js';
import EnrollPage from './+page.svelte';

const resolveMock = resolveEnroll as unknown as ReturnType<typeof vi.fn>;
const enrollMock = enroll as unknown as ReturnType<typeof vi.fn>;
const enrollLegsMock = enrollLegs as unknown as ReturnType<typeof vi.fn>;

function trip(id: number, name: string, embark: string | undefined, status = 'planned') {
	return {
		id,
		name,
		embark_date: embark,
		status,
		created_at: '2025-01-01T00:00:00Z',
		updated_at: '2025-01-01T00:00:00Z'
	};
}

const tripsOutOfDateOrder = [
	trip(12, 'Tromsø – Longyearbyen', '2025-07-10'),
	trip(13, 'Longyearbyen – Tromsø', '2025-07-20', 'cancelled'),
	trip(11, 'Bodø – Tromsø', '2025-07-01')
];

function cruiseData(overrides: Record<string, unknown> = {}) {
	return {
		kind: 'cruise',
		cruise: { id: 7, name: 'NNE 2025' },
		trips: tripsOutOfDateOrder,
		enrolled: false,
		accepted_count: 0,
		total_count: 0,
		...overrides
	};
}

beforeEach(() => {
	h.authenticated = true;
	resolveMock.mockReset().mockResolvedValue(cruiseData());
	enrollMock.mockReset().mockResolvedValue({});
	enrollLegsMock.mockReset().mockResolvedValue({ results: [] });
});

describe('enroll page — cruise legs', () => {
	it('numbers legs by embark date and marks cancelled legs unavailable', async () => {
		render(EnrollPage);
		const a = await screen.findByText('Etap 1: Bodø – Tromsø');
		const b = screen.getByText('Etap 2: Tromsø – Longyearbyen');
		const c = screen.getByText('Etap 3: Longyearbyen – Tromsø');
		expect(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
		expect(b.compareDocumentPosition(c) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

		expect(screen.getByRole('checkbox', { name: 'Etap 1: Bodø – Tromsø' })).toBeInTheDocument();
		expect(screen.getByRole('checkbox', { name: 'Etap 2: Tromsø – Longyearbyen' })).toBeInTheDocument();
		expect(screen.queryByRole('checkbox', { name: /Etap 3/ })).not.toBeInTheDocument();
		expect(screen.getByText('odwołany — niedostępny')).toBeInTheDocument();
	});

	it('submits the picked legs in one request', async () => {
		render(EnrollPage);
		await fireEvent.click(await screen.findByRole('checkbox', { name: /Etap 1/ }));
		await fireEvent.click(screen.getByRole('checkbox', { name: /Etap 2/ }));
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz się na wybrane etapy (2)' }));

		await waitFor(() => expect(enrollLegsMock).toHaveBeenCalledWith('tok', [11, 12], undefined));
		expect(enrollMock).not.toHaveBeenCalled();
		expect(await screen.findByText(/Zostałeś zapisany!/)).toBeInTheDocument();
	});

	it('falls back to a whole-cruise sign-up when no leg is picked', async () => {
		render(EnrollPage);
		await fireEvent.click(await screen.findByRole('button', { name: 'Zapisz się' }));
		await waitFor(() => expect(enrollMock).toHaveBeenCalledWith('tok', undefined));
		expect(enrollLegsMock).not.toHaveBeenCalled();
	});

	it('offers only remaining legs and no whole-cruise sign-up once a leg is booked', async () => {
		resolveMock.mockResolvedValue(
			cruiseData({
				enrolled: true,
				leg_enrollments: [{ id: 5, trip_id: 11, user_id: 1, status: 'accepted' }]
			})
		);
		render(EnrollPage);
		expect(await screen.findByText('zapisany: zaakceptowany')).toBeInTheDocument();
		expect(screen.queryByRole('checkbox', { name: /Etap 1/ })).not.toBeInTheDocument();
		expect(screen.getByRole('checkbox', { name: /Etap 2/ })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Zapisz się' })).toBeDisabled();
	});

	it('reports enrollment when every open leg is already booked', async () => {
		resolveMock.mockResolvedValue(
			cruiseData({
				enrolled: true,
				leg_enrollments: [
					{ id: 5, trip_id: 11, user_id: 1, status: 'pending' },
					{ id: 6, trip_id: 12, user_id: 1, status: 'pending' }
				]
			})
		);
		render(EnrollPage);
		expect(await screen.findByText(/Jesteś już zapisany/)).toBeInTheDocument();
		expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
	});

	it('hides leg checkboxes from signed-out visitors', async () => {
		h.authenticated = false;
		render(EnrollPage);
		await screen.findByText('Etap 1: Bodø – Tromsø');
		expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
		expect(screen.getByText('Zaloguj się, aby dołączyć')).toBeInTheDocument();
	});

	it('keeps a cruise-level sign-up visible while offering legs', async () => {
		resolveMock.mockResolvedValue(
			cruiseData({ enrolled: true, enrollment: { id: 9, cruise_id: 7, user_id: 1, status: 'pending' } })
		);
		render(EnrollPage);
		expect(await screen.findByText(/Jesteś zapisany na wydarzenie/)).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Zapisz się' })).toBeDisabled();
		await fireEvent.click(screen.getByRole('checkbox', { name: /Etap 2/ }));
		await fireEvent.input(screen.getByLabelText('Notatka (opcjonalnie)'), {
			target: { value: 'koja dziobowa' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz się na wybrane etapy (1)' }));
		await waitFor(() => expect(enrollLegsMock).toHaveBeenCalledWith('tok', [12], 'koja dziobowa'));
	});

	it('shows the server error when leg enrollment fails', async () => {
		enrollLegsMock.mockRejectedValue(new Error('trip 12 is not open for enrollment'));
		render(EnrollPage);
		await fireEvent.click(await screen.findByRole('checkbox', { name: /Etap 2/ }));
		await fireEvent.click(screen.getByRole('button', { name: /Zapisz się na wybrane etapy/ }));
		expect(await screen.findByText('trip 12 is not open for enrollment')).toBeInTheDocument();
		expect(screen.queryByText(/Zostałeś zapisany!/)).not.toBeInTheDocument();
	});

	it('shows an invalid-link message when the token does not resolve', async () => {
		resolveMock.mockRejectedValue(new Error('invalid enrollment link'));
		render(EnrollPage);
		expect(await screen.findByText('Nieprawidłowy link')).toBeInTheDocument();
		expect(screen.getByText('invalid enrollment link')).toBeInTheDocument();
	});
});

describe('enroll page — single trip', () => {
	const tripData = {
		kind: 'trip',
		trip: { id: 5, name: 'Adriatyk', embark_date: '2025-05-01', captain_name: 'Nowak' },
		enrolled: false,
		accepted_count: 1,
		total_count: 2
	};

	it('enrolls in the trip without leg checkboxes', async () => {
		resolveMock.mockResolvedValue(tripData);
		render(EnrollPage);
		expect(await screen.findByText('Adriatyk')).toBeInTheDocument();
		expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz się' }));
		await waitFor(() => expect(enrollMock).toHaveBeenCalledWith('tok', undefined));
	});

	it('reports an existing trip enrollment', async () => {
		resolveMock.mockResolvedValue({
			...tripData,
			enrolled: true,
			enrollment: { id: 3, trip_id: 5, user_id: 1, status: 'waitlisted' }
		});
		render(EnrollPage);
		expect(await screen.findByText(/Jesteś już zapisany/)).toBeInTheDocument();
		expect(screen.getByText('lista rezerwowa')).toBeInTheDocument();
	});
});
