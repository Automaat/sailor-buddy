import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

const h = vi.hoisted(() => ({
	authenticated: true,
	membership: 'czlonek' as 'czlonek' | 'sympatyk' | 'gosc' | undefined
}));

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
		},
		get user() {
			return h.membership
				? {
						membership: h.membership,
						name: 'Jan Kowalski',
						phone: '+48 600 100 200',
						patent_type: 'zeglarz_jachtowy',
						patent_number: 'ZJ-1'
					}
				: null;
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
	h.membership = 'czlonek';
	resolveMock.mockReset().mockResolvedValue(cruiseData());
	enrollMock.mockReset().mockResolvedValue({});
	enrollLegsMock.mockReset().mockResolvedValue({ results: [] });
});

describe('enroll page — profile summary', () => {
	it('shows who is enrolling with a link to edit the profile', async () => {
		render(EnrollPage);
		const summary = await screen.findByTestId('enroll-as');
		expect(summary).toHaveTextContent('Jan Kowalski');
		expect(summary).toHaveTextContent('+48 600 100 200 · Żeglarz Jachtowy · ZJ-1');
		expect(screen.getByRole('link', { name: 'Edytuj' })).toHaveAttribute('href', '/profile');
	});
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

	it('offers no whole-cruise sign-up: a leg must be picked', async () => {
		render(EnrollPage);
		const submit = await screen.findByRole('button', { name: 'Zapisz się' });
		expect(submit).toBeDisabled();
		await fireEvent.click(submit);
		expect(enrollMock).not.toHaveBeenCalled();
		expect(enrollLegsMock).not.toHaveBeenCalled();
		expect(screen.queryByText(/całe\s+wydarzenie/)).not.toBeInTheDocument();
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
		expect(screen.getByRole('link', { name: 'Zapisz się' })).toHaveAttribute(
			'href',
			'/login?redirect=%2Fenroll%2Ftok'
		);
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

function tripData(maxCrew: number | undefined, accepted: number) {
	return {
		kind: 'trip',
		trip: { id: 1, name: 'Rejs Bałtyk', max_crew: maxCrew },
		enrolled: false,
		accepted_count: accepted,
		total_count: accepted + 2,
		trip_accepted: [{ trip_id: 1, accepted_count: accepted }]
	};
}

describe('enroll page — free places', () => {
	it('shows the crew limit minus accepted enrollments for a trip', async () => {
		resolveMock.mockResolvedValue(tripData(10, 3));
		render(EnrollPage);
		expect(await screen.findByTestId('free-places')).toHaveTextContent('Wolne miejsca: 7');
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
	});

	it('counts cruise enrollments assigned to the trip when the API sends them', async () => {
		resolveMock.mockResolvedValue({
			...tripData(10, 2),
			trip_accepted: [{ trip_id: 1, accepted_count: 5 }]
		});
		render(EnrollPage);
		expect(await screen.findByTestId('free-places')).toHaveTextContent('Wolne miejsca: 5');
	});

	it('shows nothing for a trip when the accepted count is missing', async () => {
		resolveMock.mockResolvedValue({ ...tripData(10, 2), trip_accepted: undefined });
		render(EnrollPage);
		await screen.findByText('Rejs Bałtyk');
		expect(screen.queryByTestId('free-places')).not.toBeInTheDocument();
	});

	it('shows nothing for cruise legs when accepted counts are missing', async () => {
		resolveMock.mockResolvedValue(
			cruiseData({ trips: [{ ...trip(1, 'Ilmatar', '2025-07-01'), max_crew: 10 }] })
		);
		render(EnrollPage);
		await screen.findByText('Etap 1: Ilmatar');
		expect(screen.queryByTestId('free-places')).not.toBeInTheDocument();
	});

	it('shows free places per cruise leg from its accepted count', async () => {
		resolveMock.mockResolvedValue(
			cruiseData({
				trips: [
					{ ...trip(1, 'Ilmatar', '2025-07-01'), max_crew: 10 },
					{ ...trip(2, 'Kliper', '2025-07-02'), max_crew: 8 },
					{ ...trip(3, 'Bez limitu', '2025-07-03'), max_crew: undefined },
					{ ...trip(4, 'Zero', '2025-07-04'), max_crew: 0 }
				],
				trip_accepted: [
					{ trip_id: 1, accepted_count: 4 },
					{ trip_id: 2, accepted_count: 0 },
					{ trip_id: 3, accepted_count: 2 },
					{ trip_id: 4, accepted_count: 0 }
				]
			})
		);
		render(EnrollPage);
		const leg = async (text: string) => (await screen.findByText(text)).closest('label');
		expect(await leg('Etap 1: Ilmatar')).toHaveTextContent('wolne miejsca: 6');
		expect(await leg('Etap 2: Kliper')).toHaveTextContent('wolne miejsca: 8');
		expect(await leg('Etap 3: Bez limitu')).not.toHaveTextContent(/wolne miejsca/i);
		expect(await leg('Etap 4: Zero')).not.toHaveTextContent(/wolne miejsca/i);
		expect(screen.getAllByTestId('free-places')).toHaveLength(2);
	});
});

describe('enroll page — prices', () => {
	const nne = cruiseData({
		cruise: {
			id: 7,
			name: 'NNE 2025',
			currency: 'PLN',
			cost_per_person: 1000,
			cost_per_person_non_member: 1250,
			yacht_fund: 400,
			included: ['jacht', 'skipper'],
			not_included: ['dojazd']
		}
	});

	it.each([
		['czlonek', '1000 PLN'],
		['sympatyk', '1000 PLN'],
		['gosc', '1250 PLN']
	] as const)('shows %s their cruise price and what it covers', async (membership, want) => {
		h.membership = membership;
		resolveMock.mockResolvedValue(nne);
		render(EnrollPage);
		expect(await screen.findByTestId('your-price')).toHaveTextContent(`Twoja cena: ${want}`);
		expect(screen.getByText('Kasa jachtowa (szacunkowo)').nextElementSibling).toHaveTextContent('400 PLN');
		expect(screen.getByTestId('price-included')).toHaveTextContent('skipper');
		expect(screen.getByTestId('price-not-included')).toHaveTextContent('dojazd');
	});

	it('shows both tiers to a signed-out visitor', async () => {
		h.authenticated = false;
		h.membership = undefined;
		resolveMock.mockResolvedValue(nne);
		render(EnrollPage);
		expect(await screen.findByText('Goście')).toBeInTheDocument();
		expect(screen.queryByTestId('your-price')).not.toBeInTheDocument();
	});

	it('shows a guest the only price of a trip', async () => {
		h.membership = 'gosc';
		resolveMock.mockResolvedValue({
			kind: 'trip',
			trip: { id: 5, name: 'Adriatyk', currency: 'EUR', cost_per_person: 760, deposit: 200 },
			enrolled: false,
			accepted_count: 0,
			total_count: 0
		});
		render(EnrollPage);
		expect(await screen.findByTestId('your-price')).toHaveTextContent('Twoja cena: 760 EUR');
		expect(screen.getByText('Kaucja (zwrotna)').nextElementSibling).toHaveTextContent('200 EUR');
	});

	it('shows no price section for an unpriced cruise', async () => {
		render(EnrollPage);
		expect(await screen.findByText('NNE 2025')).toBeInTheDocument();
		expect(screen.queryByLabelText('Cena')).not.toBeInTheDocument();
	});
});

describe('enroll page — cruise logistics', () => {
	it('shows travel notes, minimum age and the group chat link', async () => {
		resolveMock.mockResolvedValue(
			cruiseData({
				cruise: {
					id: 7,
					name: 'NNE 2025',
					travel_notes: 'Tanie loty do Katanii',
					min_age: 18,
					group_chat_url: 'https://discord.gg/nne'
				}
			})
		);
		render(EnrollPage);

		expect(await screen.findByText('Tanie loty do Katanii')).toBeInTheDocument();
		expect(screen.getByText('18+')).toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'Czat grupy' })).toHaveAttribute('href', 'https://discord.gg/nne');
	});

	it('shows no logistics when none are set', async () => {
		render(EnrollPage);

		await screen.findByText('Etap 1: Bodø – Tromsø');
		expect(screen.queryByTestId('cruise-logistics')).not.toBeInTheDocument();
		expect(screen.queryByRole('link', { name: 'Czat grupy' })).not.toBeInTheDocument();
	});

	it('does not link a group chat URL that is not http or https', async () => {
		resolveMock.mockResolvedValue(
			cruiseData({ cruise: { id: 7, name: 'NNE 2025', group_chat_url: 'javascript:alert(1)' } })
		);
		render(EnrollPage);

		await screen.findByText('Etap 1: Bodø – Tromsø');
		expect(screen.queryByRole('link', { name: 'Czat grupy' })).not.toBeInTheDocument();
	});
});
