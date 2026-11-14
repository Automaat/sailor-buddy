import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { makeVoyage } from '#lib/test-utils.js';
import type { Yacht } from '#lib/api/types.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/api/routes.js', () => ({
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
	listYachts: vi.fn(),
	createYacht: vi.fn(),
	getCruise: vi.fn(() => Promise.resolve(null)),
	geocode: vi.fn(() => Promise.resolve([]))
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: false, user: { id: 7, role: 'member', person_id: 70 } }
}));

import { auth } from '#lib/stores/auth.svelte.js';
import {
	addVoyagePort,
	addVoyageYacht,
	createYacht,
	getVoyage,
	listAssignableCrew,
	listVoyageCrew,
	listVoyageOpinions,
	listVoyagePorts,
	listYachts,
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

const mockAuth = auth as unknown as { isAdmin: boolean; user: { id: number; role: string; person_id?: number } };

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
	vi.mocked(listYachts).mockResolvedValue([yacht({ yacht_type: 'Bavaria 46' })]);
});

describe('voyage page record', () => {
	it('shows the yacht, track link, card and a name-only anchorage route', async () => {
		vi.mocked(getVoyage).mockResolvedValue(
			makeVoyage({
				id: 5,
				yacht_id: 3,
				track_url: 'https://example.com/track/1',
				voyage_card: 'u kpt. Nowaka'
			})
		);
		vi.mocked(listVoyagePorts).mockResolvedValue([
			{ id: 1, voyage_id: 5, name: 'Gdynia', latitude: 54.5, longitude: 18.5, anchorage: false, position: 0, created_at: '' },
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
	});

	it('lets the captain add a name-only anchorage port', async () => {
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5, captain_person_id: 70 }));
		vi.mocked(addVoyagePort).mockResolvedValue({
			id: 9, voyage_id: 5, name: 'Zatoka Puck', anchorage: true, position: 0, created_at: ''
		});
		await renderPage();

		await fireEvent.input(await screen.findByLabelText('Odwiedzone porty'), { target: { value: 'Zatoka Puck' } });
		await fireEvent.click(screen.getByLabelText('⚓ Kotwicowisko'));
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj bez lokalizacji na mapie' }));

		await waitFor(() => expect(addVoyagePort).toHaveBeenCalled());
		expect(vi.mocked(addVoyagePort).mock.calls[0]).toEqual([5, { name: 'Zatoka Puck', anchorage: true, position: 0 }]);
		expect(await screen.findByText('(bez mapy)')).toBeInTheDocument();
	});

	it('lets the captain add a charter yacht and log the track and card', async () => {
		vi.mocked(getVoyage).mockResolvedValue(makeVoyage({ id: 5, captain_person_id: 70 }));
		vi.mocked(addVoyageYacht).mockResolvedValue(yacht({ id: 12, name: 'Selma', yacht_type: 'Hanse 415' }));
		vi.mocked(updateVoyageLog).mockResolvedValue(undefined as never);
		await renderPage();

		await fireEvent.click(await screen.findByRole('button', { name: 'Uzupełnij po rejsie' }));
		await fireEvent.click(screen.getByRole('button', { name: '+ Nowy jacht spoza listy' }));
		await fireEvent.input(screen.getByLabelText('Typ jachtu'), { target: { value: 'Hanse 415' } });
		await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: 'Selma' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));

		await waitFor(() => expect(screen.getByLabelText('Jacht')).toHaveValue('12'));
		expect(addVoyageYacht).toHaveBeenCalledWith(5, { name: 'Selma', yacht_type: 'Hanse 415', registration_no: undefined });
		expect(createYacht).not.toHaveBeenCalled();

		await fireEvent.input(screen.getByLabelText('Link do śladu rejsu'), { target: { value: 'https://example.com/t' } });
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
			{ id: 1, voyage_id: 5, name: 'Gdynia', latitude: 54.5, longitude: 18.5, anchorage: false, position: 0, created_at: '' }
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

describe('voyage page same-route navigation', () => {
	beforeEach(() => {
		mockAuth.user = { id: 7, role: 'member', person_id: 70 };
		vi.mocked(getVoyage).mockImplementation(async (id: number) =>
			makeVoyage({ id, name: id === 5 ? 'Rejs A' : 'Rejs B', captain_person_id: 70, captain_notes: undefined })
		);
	});

	it('swaps to the second voyage and closes the log form of the first', async () => {
		vi.mocked(listVoyagePorts).mockImplementation(async (id: number) =>
			id === 5
				? [{ id: 1, voyage_id: 5, name: 'Gdynia', latitude: 54.5, longitude: 18.5, anchorage: false, position: 0, created_at: '' }]
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

	it('drops a log save that answers after a link moved to another voyage', async () => {
		let answer: () => void = () => {};
		vi.mocked(updateVoyageLog).mockReturnValue(new Promise<never>((r) => (answer = r as () => void)));
		const view = await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Uzupełnij po rejsie' }));
		await fireEvent.input(screen.getByLabelText('Notatka z rejsu'), { target: { value: 'Notatka A' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz raport' }));

		await view.rerender({ data: await runLoad('6') });
		answer();
		await Promise.resolve();
		await Promise.resolve();

		expect(screen.getByRole('heading', { name: 'Rejs B' })).toBeInTheDocument();
		expect(screen.queryByText('Notatka A')).not.toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: 'Uzupełnij po rejsie' }));
		expect(screen.getByRole('button', { name: 'Zapisz raport' })).toBeEnabled();
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
				backHref: '/cruises?tab=completed'
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
