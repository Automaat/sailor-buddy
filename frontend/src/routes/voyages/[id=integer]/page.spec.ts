import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { makeCruise, makeVoyage } from '#lib/test-utils.js';
import type { Yacht } from '#lib/api/types.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/api/routes.js', () => ({
	listCountries: async () => (await import('#lib/test-utils.js')).testCountries,
	getVoyage: vi.fn(),
	deleteVoyage: vi.fn(),
	listVoyageCrew: vi.fn(() => Promise.resolve([])),
	assignVoyageCrew: vi.fn(),
	removeVoyageCrew: vi.fn(),
	listVoyageOpinions: vi.fn(() => Promise.resolve([])),
	generateVoyageOpinion: vi.fn(),
	deleteVoyageOpinion: vi.fn(),
	downloadVoyageOpinion: vi.fn(),
	listAssignableCrew: vi.fn(() => Promise.resolve([])),
	listVoyagePorts: vi.fn(),
	addVoyagePort: vi.fn(),
	deleteVoyagePort: vi.fn(),
	reorderVoyagePorts: vi.fn(),
	updateVoyageLog: vi.fn(),
	addVoyageYacht: vi.fn(),
	listVoyageYachts: vi.fn(),
	listYachts: vi.fn(),
	createYacht: vi.fn(),
	getCruise: vi.fn(() => Promise.resolve(null)),
	geocode: vi.fn(() => Promise.resolve([])),
	publishVoyage: vi.fn(),
	unpublishVoyage: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: false, user: { id: 7, role: 'member', person_id: 70 } }
}));

import { auth } from '#lib/stores/auth.svelte.js';
import { goto } from '$app/navigation';
import {
	addVoyagePort,
	addVoyageYacht,
	createYacht,
	deleteVoyage,
	deleteVoyageOpinion,
	generateVoyageOpinion,
	getCruise,
	getVoyage,
	listAssignableCrew,
	listVoyageCrew,
	listVoyageOpinions,
	listVoyagePorts,
	listVoyageYachts,
	listYachts,
	publishVoyage,
	unpublishVoyage,
	updateVoyageLog
} from '#lib/api/routes.js';
import { ApiError } from '#lib/api/client.js';
import VoyagePage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

function runLoad(id = '5') {
	return load({ params: { id } } as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

// renderPage runs the route's load function and renders the page with its
// data, the way SvelteKit does on navigation.
async function renderPage() {
	return render(VoyagePage, { props: { data: await runLoad() } });
}

const mockAuth = auth as unknown as {
	isAdmin: boolean;
	user: { id: number; role: string; person_id?: number };
};

function yacht(overrides: Partial<Yacht> = {}): Yacht {
	return {
		id: 3,
		name: 'Zawisza',
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

beforeEach(() => {
	vi.clearAllMocks();
	mockAuth.isAdmin = false;
	mockAuth.user = { id: 7, role: 'member', person_id: 70 };
	vi.mocked(listVoyagePorts).mockResolvedValue([]);
	vi.mocked(listVoyageYachts).mockResolvedValue([yacht({ yacht_type: 'Bavaria 46' })]);
});

describe('voyage page record', () => {
	it("names the voyage's countries in Polish", async () => {
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5, countries: ['DK'] }));
		await renderPage();

		expect(await screen.findByText(/· Dania/)).toBeInTheDocument();
	});

	it('shows the yacht, track link, card and a name-only anchorage route', async () => {
		vi.mocked(getVoyage).mockResolvedValue(
			makeVoyage({
				id: 5,
				yacht_id: 3,
				yacht: { id: 3, name: 'Zawisza', yacht_type: 'Bavaria 46' },
				track_url: 'https://example.com/track/1',
				voyage_card: 'u kpt. Nowaka'
			})
		);
		vi.mocked(listVoyagePorts).mockResolvedValue([
			{
				id: 1,
				voyage_id: 5,
				name: 'Gdynia',
				latitude: 54.5,
				longitude: 18.5,
				anchorage: false,
				position: 0,
				created_at: ''
			},
			{ id: 2, voyage_id: 5, name: 'Zatoka Puck', anchorage: true, position: 1, created_at: '' }
		]);
		await renderPage();

		expect(await screen.findByText('Bavaria 46 Zawisza')).toBeInTheDocument();
		const link = screen.getByRole('link', { name: 'https://example.com/track/1' });
		expect(link).toHaveAttribute('href', 'https://example.com/track/1');
		expect(screen.getByText('u kpt. Nowaka')).toBeInTheDocument();

		const items = screen.getAllByRole('listitem');
		expect(items[0]).toHaveTextContent('1. Gdynia');
		expect(items[1]).toHaveTextContent('2.');
		expect(items[1]).toHaveTextContent('Zatoka Puck');
		expect(within(items[1]).getByLabelText('Kotwicowisko')).toBeInTheDocument();
		expect(items[1]).toHaveTextContent('(bez mapy)');
		expect(items[0]).not.toHaveTextContent('(bez mapy)');
		// A member who does not log the voyage reads its yacht from the voyage, not the yacht list.
		expect(listVoyageYachts).not.toHaveBeenCalled();
		expect(listYachts).not.toHaveBeenCalled();
	});

	it('keeps the logged yacht when the yacht list fails to load', async () => {
		vi.mocked(getVoyage).mockResolvedValue(
			makeVoyage({
				id: 5,
				captain_person_id: 70,
				yacht_id: 3,
				yacht: { id: 3, name: 'Zawisza', yacht_type: 'Bavaria 46' }
			})
		);
		vi.mocked(listVoyageYachts).mockRejectedValue(new ApiError(500, 'boom'));
		vi.mocked(updateVoyageLog).mockResolvedValue(undefined as never);
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Uzupełnij po rejsie' }));
		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Nie udało się wczytać listy jachtów: boom'
		);
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz raport' }));

		await waitFor(() => expect(screen.queryByLabelText('Jacht')).not.toBeInTheDocument());
		expect(vi.mocked(updateVoyageLog).mock.calls[0][1]).toMatchObject({ yacht_id: 3 });
		expect(screen.getByText('Bavaria 46 Zawisza')).toBeInTheDocument();
		expect(listVoyageYachts).toHaveBeenCalledWith(5);
		expect(listYachts).not.toHaveBeenCalled();
	});

	it('lets the captain add a name-only anchorage port', async () => {
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5, captain_person_id: 70 }));
		vi.mocked(addVoyagePort).mockResolvedValue({
			id: 9,
			voyage_id: 5,
			name: 'Zatoka Puck',
			anchorage: true,
			position: 0,
			created_at: ''
		});
		await renderPage();

		await fireEvent.input(await screen.findByLabelText('Odwiedzone porty'), {
			target: { value: 'Zatoka Puck' }
		});
		await fireEvent.click(screen.getByLabelText('⚓ Kotwicowisko'));
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj bez lokalizacji na mapie' }));

		await waitFor(() => expect(addVoyagePort).toHaveBeenCalled());
		expect(vi.mocked(addVoyagePort).mock.calls[0]).toEqual([
			5,
			{ name: 'Zatoka Puck', anchorage: true }
		]);
		expect(await screen.findByText('(bez mapy)')).toBeInTheDocument();
	});

	it('lets the captain add a charter yacht and log the track and card', async () => {
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5, captain_person_id: 70 }));
		vi.mocked(addVoyageYacht).mockResolvedValue(
			yacht({ id: 12, name: 'Selma', yacht_type: 'Hanse 415' })
		);
		vi.mocked(updateVoyageLog).mockResolvedValue(undefined as never);
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Uzupełnij po rejsie' }));
		await fireEvent.click(screen.getByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.input(screen.getByLabelText('Typ jachtu'), { target: { value: 'Hanse 415' } });
		await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: 'Selma' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));

		await waitFor(() => expect(screen.getByLabelText('Jacht')).toHaveValue('12'));
		expect(addVoyageYacht).toHaveBeenCalledWith(5, {
			name: 'Selma',
			yacht_type: 'Hanse 415',
			registration_no: undefined
		});
		expect(createYacht).not.toHaveBeenCalled();

		await fireEvent.input(screen.getByLabelText('Link do śladu rejsu'), {
			target: { value: 'https://example.com/t' }
		});
		await fireEvent.input(screen.getByLabelText('Karta rejsu'), { target: { value: 'w klubie' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz raport' }));

		await waitFor(() => expect(updateVoyageLog).toHaveBeenCalled());
		expect(vi.mocked(updateVoyageLog).mock.calls[0][1]).toMatchObject({
			yacht_id: 12,
			track_url: 'https://example.com/t',
			voyage_card: 'w klubie'
		});
		expect(await screen.findByRole('link', { name: 'https://example.com/t' })).toBeInTheDocument();
		expect(screen.getByText('Hanse 415 Selma')).toBeInTheDocument();
	});

	it('saves the log with the yacht still being added', async () => {
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5, captain_person_id: 70 }));
		let resolveYacht!: (y: Yacht) => void;
		vi.mocked(addVoyageYacht).mockReturnValue(new Promise((r) => (resolveYacht = r)));
		vi.mocked(updateVoyageLog).mockResolvedValue(undefined as never);
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Uzupełnij po rejsie' }));
		await fireEvent.click(screen.getByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: 'Selma' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz raport' }));
		expect(updateVoyageLog).not.toHaveBeenCalled();

		resolveYacht(yacht({ id: 12, name: 'Selma' }));
		await waitFor(() => expect(updateVoyageLog).toHaveBeenCalledTimes(1));
		expect(vi.mocked(updateVoyageLog).mock.calls[0][1]).toMatchObject({ yacht_id: 12 });
	});

	it('lists a yacht whose create ended after the log closed and reuses it', async () => {
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5, captain_person_id: 70 }));
		let resolveYacht!: (y: Yacht) => void;
		vi.mocked(addVoyageYacht).mockReturnValue(new Promise((r) => (resolveYacht = r)));
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Uzupełnij po rejsie' }));
		await fireEvent.click(screen.getByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.input(screen.getByLabelText('Typ jachtu'), { target: { value: 'Hanse 415' } });
		await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: 'Selma' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		const [, closeLog] = screen.getAllByRole('button', { name: 'Anuluj' });
		await fireEvent.click(closeLog);
		expect(screen.queryByLabelText('Jacht')).not.toBeInTheDocument();
		resolveYacht(yacht({ id: 12, name: 'Selma', yacht_type: 'Hanse 415' }));

		await fireEvent.click(await screen.findByRole('button', { name: 'Uzupełnij po rejsie' }));
		expect(await screen.findByRole('option', { name: 'Selma (Hanse 415)' })).toBeInTheDocument();
		expect(screen.getByLabelText('Jacht')).toHaveValue('0');

		await fireEvent.click(screen.getByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.input(screen.getByLabelText('Typ jachtu'), { target: { value: 'Hanse 415' } });
		await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: 'Selma' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));

		expect(screen.getByLabelText('Jacht')).toHaveValue('12');
		expect(screen.getAllByRole('option', { name: 'Selma (Hanse 415)' })).toHaveLength(1);
		expect(addVoyageYacht).toHaveBeenCalledTimes(1);
	});

	it.each([
		['another person captains', 9],
		['the captain person id only matches the account id', 7]
	])('hides the log and port editing from other members: %s', async (_, captain) => {
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5, captain_person_id: captain }));
		await renderPage();

		await screen.findByText('Odwiedzone porty (0)');
		expect(screen.queryByRole('button', { name: 'Uzupełnij po rejsie' })).not.toBeInTheDocument();
		expect(screen.queryByLabelText('Odwiedzone porty')).not.toBeInTheDocument();
	});
});

describe('voyage page load errors', () => {
	it('names a failed crew list instead of showing no crew, and renders the ports', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		mockAuth.isAdmin = true;
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5 }));
		vi.mocked(listVoyageCrew).mockRejectedValueOnce(new Error('500'));
		vi.mocked(listVoyagePorts).mockResolvedValue([
			{
				id: 1,
				voyage_id: 5,
				name: 'Gdynia',
				latitude: 54.5,
				longitude: 18.5,
				anchorage: false,
				position: 0,
				created_at: ''
			}
		]);
		await renderPage();

		expect(await screen.findByText(/Nie udało się wczytać: załoga\./)).toBeInTheDocument();
		expect(screen.getByText('Odwiedzone porty (1)')).toBeInTheDocument();
	});

	it('names no admin-only section for a member', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5 }));
		vi.mocked(listVoyageCrew).mockRejectedValueOnce(new Error('500'));
		vi.mocked(listVoyageOpinions).mockRejectedValueOnce(new Error('500'));
		vi.mocked(listAssignableCrew).mockRejectedValueOnce(new Error('500'));
		await renderPage();

		await screen.findByText('Odwiedzone porty (0)');
		expect(listVoyageCrew).toHaveBeenCalledWith(5);
		expect(screen.queryByText(/Nie udało się wczytać/)).not.toBeInTheDocument();
	});

	it('shows no load banner when every section loads', async () => {
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5 }));
		await renderPage();

		await screen.findByText('Odwiedzone porty (0)');
		expect(screen.queryByText(/Nie udało się wczytać/)).not.toBeInTheDocument();
	});
});

describe('voyage page delete', () => {
	beforeEach(() => {
		mockAuth.isAdmin = true;
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5, cruise_id: 1 }));
	});

	it('goes back to the event after the voyage is deleted', async () => {
		vi.mocked(deleteVoyage).mockResolvedValue(undefined);
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));

		await waitFor(() => expect(goto).toHaveBeenCalledWith('/cruises/1'));
		expect(deleteVoyage).toHaveBeenCalledWith(5);
	});

	it('shows the error and stays on the voyage when deletion fails', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		vi.mocked(deleteVoyage).mockRejectedValue(new ApiError(409, 'voyage has crew'));
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));

		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Nie udało się usunąć rejsu: voyage has crew'
		);
		expect(screen.getByRole('heading', { name: 'Rejs Bałtyk' })).toBeInTheDocument();
		expect(goto).not.toHaveBeenCalled();
	});
});

describe('voyage page same-route navigation', () => {
	beforeEach(() => {
		mockAuth.user = { id: 7, role: 'member', person_id: 70 };
		vi.mocked(getVoyage).mockImplementation(async (id: number) =>
			makeVoyage({
				id,
				name: id === 5 ? 'Rejs A' : 'Rejs B',
				captain_person_id: 70,
				captain_notes: undefined
			})
		);
	});

	it('swaps to the second voyage and closes the log form of the first', async () => {
		vi.mocked(listVoyagePorts).mockImplementation(async (id: number) =>
			id === 5
				? [
						{
							id: 1,
							voyage_id: 5,
							name: 'Gdynia',
							latitude: 54.5,
							longitude: 18.5,
							anchorage: false,
							position: 0,
							created_at: ''
						}
					]
				: []
		);
		vi.mocked(updateVoyageLog).mockRejectedValue(new Error('zapis odrzucony'));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Uzupełnij po rejsie' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz raport' }));
		expect(await screen.findByText('zapis odrzucony')).toBeInTheDocument();

		await view.rerender({ data: await runLoad('6') });

		expect(screen.getByRole('heading', { name: 'Rejs B' })).toBeInTheDocument();
		expect(screen.getByText('Odwiedzone porty (0)')).toBeInTheDocument();
		await waitFor(() => expect(screen.queryByText('zapis odrzucony')).not.toBeInTheDocument());
		expect(screen.queryByRole('button', { name: 'Zapisz raport' })).not.toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Uzupełnij po rejsie' })).toBeInTheDocument();
	});

	it.each(['5', '6'])('drops a log save after a new load for voyage %s', async (nextId) => {
		let answer: () => void = () => {};
		vi.mocked(updateVoyageLog).mockReturnValue(
			new Promise<never>((r) => (answer = r as () => void))
		);
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Uzupełnij po rejsie' }));
		await fireEvent.input(screen.getByLabelText('Notatka z rejsu'), {
			target: { value: 'Notatka A' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz raport' }));

		await view.rerender({ data: await runLoad(nextId) });
		answer();
		await Promise.resolve();
		await Promise.resolve();

		expect(
			screen.getByRole('heading', { name: nextId === '5' ? 'Rejs A' : 'Rejs B' })
		).toBeInTheDocument();
		expect(screen.queryByText('Notatka A')).not.toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Uzupełnij po rejsie' }));
		expect(screen.getByRole('button', { name: 'Zapisz raport' })).toBeEnabled();
	});
});

describe('voyage page same-route navigation: port picker', () => {
	it('clears a failed port add and the typed name from the previous voyage', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		vi.mocked(getVoyage).mockImplementation(async (id: number) =>
			makeVoyage({ id, name: id === 5 ? 'Rejs A' : 'Rejs B', captain_person_id: 70 })
		);
		vi.mocked(addVoyagePort).mockRejectedValue(new Error('500'));
		const view = await renderPage();
		await fireEvent.input(await screen.findByLabelText('Odwiedzone porty'), {
			target: { value: 'Hel' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj bez lokalizacji na mapie' }));
		expect(await screen.findByText('Nie udało się dodać portu')).toBeInTheDocument();

		await view.rerender({ data: await runLoad('6') });

		expect(screen.getByRole('heading', { name: 'Rejs B' })).toBeInTheDocument();
		expect(screen.queryByText('Nie udało się dodać portu')).not.toBeInTheDocument();
		expect(screen.getByLabelText('Odwiedzone porty')).toHaveValue('');
	});
});

describe('voyage page load', () => {
	it('throws a 404 with the voyage copy for +error.svelte', async () => {
		vi.mocked(getVoyage).mockRejectedValue(new ApiError(404, 'not found'));

		await expect(runLoad()).rejects.toMatchObject({
			status: 404,
			body: {
				message: 'Ten zrealizowany rejs nie istnieje lub nie masz do niego dostępu.',
				title: 'Nie znaleziono rejsu',
				backHref: '/rejsy'
			}
		});
	});

	it('passes other failures on to +error.svelte', async () => {
		const boom = new Error('boom');
		vi.mocked(getVoyage).mockRejectedValue(boom);

		await expect(runLoad()).rejects.toBe(boom);
	});

	it('loads the voyage named by the URL', async () => {
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 9 }));
		await runLoad('9');

		expect(getVoyage).toHaveBeenCalledWith(9);
		expect(listVoyagePorts).toHaveBeenCalledWith(9);
		expect(listVoyageCrew).toHaveBeenCalledWith(9);
	});
});

describe('voyage page back link', () => {
	it('links back to its edition above the title', async () => {
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5, cruise_id: 1 }));
		vi.mocked(getCruise).mockResolvedValue(makeCruise({ id: 1, name: 'Bałtyk 2027' }));
		await renderPage();

		const back = await screen.findByTestId('back-link');
		expect(back).toHaveAttribute('href', '/cruises/1');
		expect(back).toHaveAccessibleName('Część wydarzenia: Bałtyk 2027');
	});
});

describe('voyage page publication', () => {
	const at = '2026-03-01T10:00:00Z';
	const authState = auth as unknown as { isAdmin: boolean; user: Record<string, unknown> };
	const header = () => screen.getByRole('heading', { name: 'Rejs Bałtyk' }).parentElement!;
	const voyage = (overrides: Parameters<typeof makeVoyage>[0] = {}) =>
		makeVoyage({ id: 5, cruise_id: 1, ...overrides });

	beforeEach(() => {
		authState.isAdmin = true;
		authState.user = { id: 1, role: 'admin', is_captain: false };
		vi.mocked(getCruise).mockResolvedValue(makeCruise({ id: 1, published_at: at }));
	});

	it('publishes a hidden voyage and drops the badge', async () => {
		vi.mocked(getVoyage).mockResolvedValue(voyage());
		vi.mocked(publishVoyage).mockResolvedValue(voyage({ published_at: at }));
		await renderPage();
		expect(within(header()).getByText('Ukryte')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Opublikuj' }));

		expect(publishVoyage).toHaveBeenCalledWith(5);
		await waitFor(() => expect(within(header()).queryByText('Ukryte')).not.toBeInTheDocument());
		expect(screen.getByRole('button', { name: 'Ukryj' })).toBeInTheDocument();
	});

	it('clears the badge when publishing also publishes a standalone cruise', async () => {
		vi.mocked(getCruise).mockResolvedValue(makeCruise({ id: 1 }));
		vi.mocked(getVoyage).mockResolvedValue(voyage());
		vi.mocked(publishVoyage).mockResolvedValue(voyage({ published_at: at }));
		await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: 'Opublikuj' }));

		await waitFor(() => expect(within(header()).queryByText('Ukryte')).not.toBeInTheDocument());
	});

	it('hides a published voyage', async () => {
		vi.mocked(getVoyage).mockResolvedValue(voyage({ published_at: at }));
		vi.mocked(unpublishVoyage).mockResolvedValue(voyage());
		await renderPage();
		expect(within(header()).queryByText('Ukryte')).not.toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Ukryj' }));

		expect(unpublishVoyage).toHaveBeenCalledWith(5);
		expect(await within(header()).findByText('Ukryte')).toBeInTheDocument();
	});

	it('keeps the badge and shows the error when publishing fails', async () => {
		const quiet = vi.spyOn(console, 'error').mockImplementation(() => {});
		vi.mocked(getVoyage).mockResolvedValue(voyage());
		vi.mocked(publishVoyage).mockRejectedValue(new ApiError(500, 'failed to publish voyage'));
		await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: 'Opublikuj' }));

		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Nie udało się opublikować rejsu: failed to publish voyage'
		);
		expect(within(header()).getByText('Ukryte')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Opublikuj' })).toBeEnabled();
		quiet.mockRestore();
	});

	it('shows the captain group the badge but no publish controls', async () => {
		authState.isAdmin = false;
		authState.user = { id: 3, role: 'member', is_captain: true };
		vi.mocked(getVoyage).mockResolvedValue(voyage());
		await renderPage();

		expect(within(header()).getByText('Ukryte')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Opublikuj' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Ukryj' })).not.toBeInTheDocument();
	});

	it('shows a member neither the badge nor publish controls', async () => {
		authState.isAdmin = false;
		authState.user = { id: 7, role: 'member', is_captain: false, person_id: 70 };
		vi.mocked(getVoyage).mockResolvedValue(voyage());
		await renderPage();

		expect(screen.queryByText('Ukryte')).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Opublikuj' })).not.toBeInTheDocument();
		expect(screen.queryByRole('button', { name: 'Ukryj' })).not.toBeInTheDocument();
	});
});

describe('voyage page for the voyage captain', () => {
	const anna = {
		id: 11,
		voyage_id: 5,
		crew_member_id: 3,
		full_name: 'Anna Nowak',
		role: 'zalogant',
		created_at: '2026-01-01T00:00:00Z'
	};
	const opinion = {
		id: 21,
		voyage_id: 5,
		crew_member_id: 3,
		full_name: 'Anna Nowak',
		file_format: 'pdf',
		created_at: '2026-01-01T00:00:00Z'
	};

	beforeEach(() => {
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5, captain_person_id: 70 }));
		vi.mocked(listVoyageCrew).mockResolvedValue([anna]);
		vi.mocked(listVoyageOpinions).mockResolvedValue([opinion]);
		vi.mocked(listAssignableCrew).mockResolvedValue([
			{ id: 4, full_name: 'Piotr Maj', has_account: false }
		] as Awaited<ReturnType<typeof listAssignableCrew>>);
	});

	it('shows the crew read-only', async () => {
		await renderPage();

		const crew = await screen.findByTestId('voyage-crew');
		expect(within(crew).getByRole('heading', { name: 'Załoga (1)' })).toBeInTheDocument();
		expect(within(crew).getByText('Anna Nowak')).toBeInTheDocument();
		expect(within(crew).getByText('zalogant')).toBeInTheDocument();
		expect(within(crew).queryByRole('combobox')).not.toBeInTheDocument();
		expect(within(crew).queryByRole('button', { name: 'Dodaj' })).not.toBeInTheDocument();
		expect(within(crew).queryByRole('button', { name: 'Usuń' })).not.toBeInTheDocument();
	});

	it('generates an opinion for a crew member', async () => {
		vi.mocked(generateVoyageOpinion).mockResolvedValue(opinion);
		await renderPage();

		const opinions = await screen.findByTestId('voyage-opinions');
		await fireEvent.change(within(opinions).getByLabelText('Załogant'), {
			target: { value: '3' }
		});
		await fireEvent.click(within(opinions).getByRole('button', { name: 'Generuj' }));

		await waitFor(() =>
			expect(generateVoyageOpinion).toHaveBeenCalledWith(5, { crew_member_id: 3, format: 'pdf' })
		);
	});

	it('deletes an opinion after confirmation', async () => {
		vi.mocked(deleteVoyageOpinion).mockResolvedValue(undefined);
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
		await renderPage();

		const opinions = await screen.findByTestId('voyage-opinions');
		await fireEvent.click(within(opinions).getByRole('button', { name: 'Usuń' }));

		await waitFor(() => expect(deleteVoyageOpinion).toHaveBeenCalledWith(5, 21));
		await waitFor(() =>
			expect(within(opinions).getByText('Brak wygenerowanych opinii.')).toBeInTheDocument()
		);
		confirmSpy.mockRestore();
	});

	it('names a failed crew list to the captain', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		vi.mocked(listVoyageCrew).mockRejectedValueOnce(new Error('500'));
		vi.mocked(listAssignableCrew).mockRejectedValueOnce(new Error('500'));
		await renderPage();

		expect(await screen.findByText(/Nie udało się wczytać: załoga\./)).toBeInTheDocument();
		expect(screen.queryByText(/lista osób/)).not.toBeInTheDocument();
	});

	it('shows an admin the crew form', async () => {
		mockAuth.isAdmin = true;
		mockAuth.user = { id: 1, role: 'admin', person_id: 99 };
		await renderPage();

		const crew = await screen.findByTestId('voyage-crew');
		expect(within(crew).getByRole('button', { name: 'Dodaj' })).toBeInTheDocument();
		expect(within(crew).getByRole('button', { name: 'Usuń' })).toBeInTheDocument();
		expect(screen.getByTestId('voyage-opinions')).toBeInTheDocument();
	});

	it('shows another member neither the crew nor the opinions form', async () => {
		mockAuth.user = { id: 8, role: 'member', person_id: 71 };
		await renderPage();

		await screen.findByText('Odwiedzone porty (0)');
		expect(screen.queryByTestId('voyage-crew')).not.toBeInTheDocument();
		expect(screen.queryByTestId('voyage-opinions')).not.toBeInTheDocument();
	});
});

describe('voyage page long text', () => {
	it('lets a long unbroken name shrink in the header and the crew list', async () => {
		const long = 'W'.repeat(120);
		mockAuth.isAdmin = true;
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5, name: long }));
		vi.mocked(listVoyageCrew).mockResolvedValue([
			{ id: 1, crew_member_id: 3, full_name: `N${long}`, role: 'zalogant' }
		] as Awaited<ReturnType<typeof listVoyageCrew>>);
		await renderPage();

		const heading = await screen.findByRole('heading', { name: long });
		expect(heading.parentElement?.parentElement).toHaveClass('min-w-0');
		const name = await screen.findByText(`N${long}`, { selector: 'span' });
		expect(name).toHaveClass('min-w-0');
		expect(name.parentElement).toHaveClass('min-w-0');
	});
});
