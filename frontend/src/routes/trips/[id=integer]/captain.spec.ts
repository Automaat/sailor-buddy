import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { makeCruise, makeTrip } from '#lib/test-utils.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/api/routes.js', () => ({
	getTrip: vi.fn(),
	deleteTrip: vi.fn(),
	listTripCrew: vi.fn(),
	removeTripCrew: vi.fn(),
	updateTripCrewRole: vi.fn(),
	cancelTrip: vi.fn(),
	completeTrip: vi.fn(),
	getCruise: vi.fn(),
	generateTripEnrollToken: vi.fn(),
	clearTripEnrollToken: vi.fn(),
	getTripSignUp: vi.fn(() => Promise.resolve(null)),
	signUpForTrip: vi.fn(),
	updateTripSignUpNote: vi.fn(),
	withdrawTripSignUp: vi.fn(),
	publishTrip: vi.fn(),
	unpublishTrip: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: false, user: { id: 2, role: 'member' } }
}));

import { auth } from '#lib/stores/auth.svelte.js';
import { goto } from '$app/navigation';
import { completeTrip, getCruise, getTrip, listTripCrew } from '#lib/api/routes.js';
import TripDetailPage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

const mockAuth = auth as unknown as {
	isAdmin: boolean;
	user: { id: number; role: string; person_id?: number };
};

const CAPTAIN = 70;

const anna = {
	id: 1,
	crew_member_id: 3,
	full_name: 'Anna Nowak',
	role: 'zalogant',
	email: 'anna@example.dev',
	phone: '+48 600 100 200',
	emergency_contact_name: 'Ewa Nowak',
	emergency_contact_phone: '+48 600 300 400',
	created_at: '2026-01-01T00:00:00Z'
};

async function renderLeg(trip: Parameters<typeof makeTrip>[0] = {}) {
	vi.mocked(getTrip).mockResolvedValue(
		makeTrip({
			id: 50,
			name: 'Jacht A',
			captain_person_id: CAPTAIN,
			embark_date: '2026-11-05',
			disembark_date: '2026-11-12',
			...trip
		})
	);
	const data = (await load({ params: { id: '50' } } as unknown as Parameters<
		typeof load
	>[0])) as PageData;
	render(TripDetailPage, { props: { data } });
	await screen.findByRole('heading', { name: 'Jacht A' });
}

function signInAs(personId: number | undefined, admin = false) {
	mockAuth.isAdmin = admin;
	mockAuth.user = { id: 2, role: admin ? 'admin' : 'member', person_id: personId };
}

beforeEach(() => {
	vi.useFakeTimers({ toFake: ['Date'] });
	vi.setSystemTime(new Date('2026-11-13T12:00:00Z'));
	vi.mocked(getCruise).mockResolvedValue(makeCruise({ id: 1 }));
	vi.mocked(listTripCrew).mockResolvedValue([anna]);
	vi.mocked(completeTrip).mockReset();
	signInAs(CAPTAIN);
});

afterEach(() => {
	vi.useRealTimers();
});

describe('trip page crew for the leg captain', () => {
	it('shows the crew read-only with contacts', async () => {
		await renderLeg({ disembark_date: '2026-11-20' });

		const section = screen.getByTestId('trip-crew');
		expect(within(section).getByRole('heading', { name: 'Załoga (1)' })).toBeInTheDocument();
		expect(within(section).getByText('Anna Nowak')).toBeInTheDocument();
		expect(within(section).getByText('zalogant')).toBeInTheDocument();
		const contacts = within(section).getByTestId('crew-contacts');
		expect(within(contacts).getByRole('link', { name: '+48 600 100 200' })).toHaveAttribute(
			'href',
			'tel:+48 600 100 200'
		);
		expect(within(contacts).getByRole('link', { name: 'anna@example.dev' })).toHaveAttribute(
			'href',
			'mailto:anna@example.dev'
		);
		expect(contacts).toHaveTextContent('Kontakt awaryjny: Ewa Nowak +48 600 300 400');

		expect(screen.queryByLabelText('Rola: Anna Nowak')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Usuń' })).not.toBeInTheDocument();
		expect(screen.queryByTestId('crew-hint')).not.toBeInTheDocument();
		expect(screen.queryByRole('link', { name: 'Zarządzaj zapisami' })).not.toBeInTheDocument();
		expect(screen.queryByRole('link', { name: 'Edytuj' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Więcej akcji' })).not.toBeInTheDocument();
	});

	it('names a failed crew list to the captain', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		vi.mocked(listTripCrew).mockRejectedValueOnce(new Error('500'));
		await renderLeg();

		expect(screen.getByText(/Nie udało się wczytać: załoga\./)).toBeInTheDocument();
	});

	it('shows an admin the contacts next to the editing controls', async () => {
		signInAs(99, true);
		await renderLeg({ disembark_date: '2026-11-20' });

		const section = screen.getByTestId('trip-crew');
		expect(within(section).getByLabelText('Rola: Anna Nowak')).toBeInTheDocument();
		expect(within(section).getByRole('button', { name: 'Usuń' })).toBeInTheDocument();
		expect(within(section).getByTestId('crew-contacts')).toHaveTextContent(
			'Kontakt awaryjny: Ewa Nowak +48 600 300 400'
		);
	});

	it.each([
		{ name: 'a member', personId: 71 },
		{ name: 'a member without a person', personId: undefined }
	])('shows $name no crew section', async ({ personId }) => {
		signInAs(personId);
		await renderLeg();

		expect(screen.queryByTestId('trip-crew')).not.toBeInTheDocument();
		expect(screen.queryByText('Anna Nowak')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /Zrealizuj/ })).not.toBeInTheDocument();
	});
});

describe('trip page completion by the leg captain', () => {
	it.each([
		{
			name: 'after the end date',
			now: '2026-11-13T12:00:00Z',
			disembark: '2026-11-12',
			shown: true
		},
		{ name: 'on the end date', now: '2026-11-13T12:00:00Z', disembark: '2026-11-13', shown: false },
		{
			name: 'before the end date',
			now: '2026-11-13T12:00:00Z',
			disembark: '2026-11-20',
			shown: false
		},
		{
			name: 'without an end date',
			now: '2026-11-13T12:00:00Z',
			disembark: undefined,
			shown: false
		},
		{
			name: 'just after Warsaw midnight',
			now: '2026-11-13T23:30:00Z',
			disembark: '2026-11-13',
			shown: true
		},
		{
			name: 'just before Warsaw midnight',
			now: '2026-11-13T22:30:00Z',
			disembark: '2026-11-13',
			shown: false
		}
	])('offers Zrealizuj $name: $shown', async ({ now, disembark, shown }) => {
		vi.setSystemTime(new Date(now));
		await renderLeg({ disembark_date: disembark });

		const button = screen.queryByRole('button', { name: 'Zrealizuj' });
		const prompt = screen.queryByRole('button', { name: 'Zrealizuj teraz' });
		if (shown) {
			expect(button).toBeInTheDocument();
			expect(prompt).toBeInTheDocument();
		} else {
			expect(button).not.toBeInTheDocument();
			expect(prompt).not.toBeInTheDocument();
		}
	});

	it('offers no Zrealizuj on a cancelled leg', async () => {
		await renderLeg({ status: 'cancelled' });

		expect(screen.queryByRole('button', { name: /Zrealizuj/ })).not.toBeInTheDocument();
	});

	it('completes the leg and opens the voyage', async () => {
		vi.mocked(completeTrip).mockResolvedValue({ id: 90 } as Awaited<
			ReturnType<typeof completeTrip>
		>);
		await renderLeg();

		await fireEvent.click(screen.getByRole('button', { name: 'Zrealizuj' }));
		await fireEvent.click(await screen.findByRole('button', { name: 'Zrealizuj rejs' }));

		await waitFor(() => expect(completeTrip).toHaveBeenCalledWith(50, expect.any(Object)));
		await waitFor(() => expect(goto).toHaveBeenCalledWith('/voyages/90'));
	});
});
