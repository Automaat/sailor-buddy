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

import { fireEvent, waitFor, within } from '@testing-library/svelte';
import { goto } from '$app/navigation';
import { ApiError } from '#lib/api/client.js';
import { auth } from '#lib/stores/auth.svelte.js';
import {
	getSeries,
	deleteCruise,
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
	listCruiseEnrollments
} from '#lib/api/routes.js';
import CruiseDetailPage from './+page.svelte';

const mockGet = getCruise as unknown as ReturnType<typeof vi.fn>;

function cruise(overrides: Partial<Cruise> = {}): Cruise {
	return {
		id: 7,
		name: 'Sycylia 2026',
		kind: 'sea',
		currency: 'PLN',
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

describe('cruise page logistics edge cases', () => {
	it('shows no logistics card when the only field is a non-http link', async () => {
		mockGet.mockResolvedValue(cruise({ group_chat_url: 'https://chat.example.com:99999/g' }));
		render(CruiseDetailPage);

		await screen.findByRole('heading', { name: 'Sycylia 2026' });
		expect(screen.queryByText('Informacje dla załogi')).not.toBeInTheDocument();
	});
});

const enrollmentRow = {
	id: 31,
	cruise_id: 7,
	user_id: 4,
	user_name: 'Ala Kowalska',
	user_email: 'ala@example.com',
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
		fn(listCruiseTrips).mockResolvedValue([tripRow(50, 'Jacht A', 'planned'), tripRow(51, 'Jacht B', 'cancelled')]);
		fn(listCruiseVoyages).mockResolvedValue([{ id: 9, name: 'Rejs 2025', miles: 120.4, days: 7 }]);
		fn(listCruiseEnrollments).mockResolvedValue([enrollmentRow, { ...enrollmentRow, id: 32, user_name: undefined, status: 'pending', note: undefined, trip_id: undefined, trip_name: undefined }]);
	});

	it('renders series, enrollments, trips and voyages', async () => {
		render(CruiseDetailPage);

		expect(await screen.findByRole('link', { name: 'May I Sail' })).toHaveAttribute('href', '/series/3');
		expect(screen.getByText('Ala Kowalska')).toBeInTheDocument();
		expect(screen.getByText('Nieznany')).toBeInTheDocument();
		expect(screen.getByText('Mogę być kukiem')).toBeInTheDocument();
		expect(screen.getByText('1 / 2 zaakceptowanych')).toBeInTheDocument();
		expect(screen.getByText('Planowane rejsy (2)')).toBeInTheDocument();
		expect(screen.getByText('Anulowany')).toBeInTheDocument();
		expect(screen.getByText('Zrealizowane rejsy (1)')).toBeInTheDocument();
		expect(screen.getByText('120 Mm · 7 dni')).toBeInTheDocument();
	});

	it('changes enrollment status, assigns a trip and deletes an enrollment', async () => {
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		render(CruiseDetailPage);
		await screen.findByText('Ala Kowalska');

		await fireEvent.click(screen.getAllByRole('button', { name: 'Akceptuj' })[0]);
		await waitFor(() => expect(updateCruiseEnrollmentStatus).toHaveBeenCalledWith(7, 32, 'accepted'));

		await fireEvent.click(screen.getAllByRole('button', { name: 'Rezerwa' })[0]);
		await waitFor(() => expect(updateCruiseEnrollmentStatus).toHaveBeenCalledWith(7, 31, 'waitlisted'));

		const select = screen.getAllByRole('combobox')[0];
		await fireEvent.change(select, { target: { value: '' } });
		await waitFor(() => expect(assignCruiseEnrollmentTrip).toHaveBeenCalledWith(7, expect.any(Number), undefined));

		await fireEvent.click(screen.getAllByRole('button', { name: 'Odrzuć' })[0]);
		await waitFor(() => expect(updateCruiseEnrollmentStatus).toHaveBeenCalledWith(7, expect.any(Number), 'rejected'));

		await fireEvent.click(screen.getAllByRole('button', { name: 'Usuń' }).at(-1)!);
		await waitFor(() => expect(deleteCruiseEnrollment).toHaveBeenCalledWith(7, 32));
		await waitFor(() => expect(screen.queryByText('Nieznany')).not.toBeInTheDocument());
	});

	it('toggles enrollment off and on and copies the link', async () => {
		const writeText = vi.fn();
		Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true });
		fn(generateCruiseEnrollToken).mockResolvedValue({ token: 'new' });
		render(CruiseDetailPage);

		await fireEvent.click(await screen.findByRole('button', { name: 'Kopiuj link' }));
		expect(writeText).toHaveBeenCalledWith(`${window.location.origin}/enroll/tok`);

		await fireEvent.click(screen.getByRole('button', { name: 'Wyłącz zapisy' }));
		await waitFor(() => expect(clearCruiseEnrollToken).toHaveBeenCalledWith(7));
		await fireEvent.click(await screen.findByRole('button', { name: 'Włącz zapisy' }));
		await waitFor(() => expect(generateCruiseEnrollToken).toHaveBeenCalledWith(7));
		expect(await screen.findByRole('button', { name: 'Wyłącz zapisy' })).toBeInTheDocument();
	});

	it('deletes the cruise after confirmation', async () => {
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		render(CruiseDetailPage);
		const header = (await screen.findByRole('heading', { name: 'Sycylia 2026' })).closest('div')!.parentElement!;

		await fireEvent.click(within(header).getByRole('button', { name: 'Usuń' }));
		await waitFor(() => expect(deleteCruise).toHaveBeenCalledWith(7));
		expect(goto).toHaveBeenCalledWith('/cruises');
	});

	it('keeps the page usable when actions fail', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		fn(updateCruiseEnrollmentStatus).mockRejectedValue(new Error('x'));
		fn(assignCruiseEnrollmentTrip).mockRejectedValue(new Error('x'));
		fn(deleteCruiseEnrollment).mockRejectedValue(new Error('x'));
		fn(clearCruiseEnrollToken).mockRejectedValue(new Error('x'));
		render(CruiseDetailPage);
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

describe('cruise page load errors', () => {
	it('shows not found for a 404', async () => {
		mockGet.mockRejectedValue(new ApiError(404, 'not found'));
		render(CruiseDetailPage);

		expect(await screen.findByText('Nie znaleziono wydarzenia')).toBeInTheDocument();
	});

	it('shows a generic error for other failures', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		mockGet.mockRejectedValue(new Error('boom'));
		render(CruiseDetailPage);

		expect(await screen.findByText('Nie udało się wczytać wydarzenia')).toBeInTheDocument();
	});
});

describe('cruise page price', () => {
	it.each([
		['czlonek', '820 PLN'],
		['sympatyk', '820 PLN'],
		['gosc', '870 PLN']
	] as const)('shows %s their price and the included lists', async (membership, want) => {
		mockAuth.user = { id: 2, role: 'member', membership };
		mockGet.mockResolvedValue(
			cruise({
				cost_per_person: 820,
				cost_per_person_non_member: 870,
				included: ['jacht', 'paliwo'],
				not_included: ['dojazd']
			})
		);
		render(CruiseDetailPage);
		expect(await screen.findByTestId('your-price')).toHaveTextContent(`Twoja cena: ${want}`);
		expect(screen.getByTestId('price-included')).toHaveTextContent('paliwo');
		expect(screen.getByTestId('price-not-included')).toHaveTextContent('dojazd');
	});

	it('hides the price card for an unpriced cruise', async () => {
		mockGet.mockResolvedValue(cruise({ cost_per_person: 0 }));
		render(CruiseDetailPage);
		expect(await screen.findByRole('heading', { name: 'Sycylia 2026' })).toBeInTheDocument();
		expect(screen.queryByRole('heading', { name: 'Cena' })).not.toBeInTheDocument();
	});
});
