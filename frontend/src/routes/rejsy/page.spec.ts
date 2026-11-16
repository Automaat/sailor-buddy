import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/svelte';
import { makeSeries } from '#lib/test-utils.js';
import type { StandaloneLeg } from '#lib/api/types.js';

vi.mock('#lib/api/routes.js', () => ({
	listSeries: vi.fn(),
	listStandaloneLegs: vi.fn(),
	createSeries: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin', is_captain: false } }
}));

import { listSeries, listStandaloneLegs, createSeries } from '#lib/api/routes.js';
import { auth } from '#lib/stores/auth.svelte.js';
import RejsyPage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

const mockList = vi.mocked(listSeries);
const mockStandalone = vi.mocked(listStandaloneLegs);
const mockCreate = vi.mocked(createSeries);
const mockAuth = auth as unknown as { isAdmin: boolean; user: { is_captain: boolean } };

const published = '2026-01-01T00:00:00Z';

function entry(overrides: Partial<StandaloneLeg> = {}): StandaloneLeg {
	return {
		kind: 'trip',
		id: 1,
		name: 'Chorwacja',
		cancelled: false,
		cruise_id: 10,
		cruise_name: 'Chorwacja',
		embark_date: '2099-07-01',
		disembark_date: '2099-07-08',
		published_at: published,
		cruise_published_at: published,
		...overrides
	};
}

async function renderPage() {
	const data = (await load({} as Parameters<typeof load>[0])) as PageData;
	return render(RejsyPage, { props: { data } });
}

function asMember() {
	mockAuth.isAdmin = false;
	mockAuth.user.is_captain = false;
}

beforeEach(() => {
	mockList.mockReset().mockResolvedValue([]);
	mockStandalone.mockReset().mockResolvedValue([]);
	mockCreate.mockReset();
	vi.spyOn(console, 'error').mockImplementation(() => {});
	mockAuth.isAdmin = true;
	mockAuth.user.is_captain = false;
});

describe('Rejsy page', () => {
	it('is titled Rejsy', async () => {
		await renderPage();
		expect(screen.getByRole('heading', { level: 1, name: 'Rejsy' })).toBeInTheDocument();
	});

	it('shows the empty state when there are no series and no standalone legs', async () => {
		await renderPage();
		expect(screen.getByText('Brak rejsów')).toBeInTheDocument();
		expect(screen.queryByRole('region', { name: 'Bez serii' })).not.toBeInTheDocument();
	});

	it('links each series to its page', async () => {
		mockList.mockResolvedValue([
			makeSeries({ id: 4, name: 'AGH Winter Sail Expedition', description: 'Od 2012' })
		]);
		await renderPage();
		const link = screen.getByText('AGH Winter Sail Expedition').closest('a');
		expect(link).toHaveAttribute('href', '/rejsy/4');
		expect(screen.getByText('Od 2012')).toBeInTheDocument();
	});

	it("shows each series' kind on its card", async () => {
		mockList.mockResolvedValue([
			makeSeries({ id: 4, name: 'Mazury', kind: 'inland' }),
			makeSeries({ id: 5, name: 'Bałtyk', kind: 'sea' })
		]);
		await renderPage();
		expect(screen.getByText('Mazury').closest('a')).toHaveTextContent('Śródlądowy');
		expect(screen.getByText('Bałtyk').closest('a')).toHaveTextContent('Morski');
	});

	it('lets an admin create a series', async () => {
		mockCreate.mockResolvedValue(makeSeries({ id: 7, name: 'Navigare Necesse Est' }));
		await renderPage();

		await fireEvent.click(screen.getByRole('button', { name: '+ Dodaj serię' }));
		await fireEvent.input(screen.getByLabelText('Nazwa *'), {
			target: { value: 'Navigare Necesse Est' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj' }));

		expect(mockCreate).toHaveBeenCalledWith({
			name: 'Navigare Necesse Est',
			description: undefined,
			kind: 'sea'
		});
		expect(await screen.findByText('Navigare Necesse Est')).toBeInTheDocument();
	});

	it('creates a series of the picked kind', async () => {
		mockCreate.mockResolvedValue(makeSeries({ id: 8, name: 'Mazury', kind: 'inland' }));
		await renderPage();

		await fireEvent.click(screen.getByRole('button', { name: '+ Dodaj serię' }));
		expect(screen.getByLabelText('Rodzaj rejsów')).toHaveValue('sea');
		await fireEvent.input(screen.getByLabelText('Nazwa *'), { target: { value: 'Mazury' } });
		await fireEvent.change(screen.getByLabelText('Rodzaj rejsów'), {
			target: { value: 'inland' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj' }));

		expect(mockCreate).toHaveBeenCalledWith({
			name: 'Mazury',
			description: undefined,
			kind: 'inland'
		});
		expect((await screen.findByText('Mazury')).closest('a')).toHaveTextContent('Śródlądowy');
	});

	it('shows the error and keeps the form when creating fails', async () => {
		mockCreate.mockRejectedValue(new Error('boom'));
		await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: '+ Dodaj serię' }));
		await fireEvent.input(screen.getByLabelText('Nazwa *'), { target: { value: 'X' } });
		await fireEvent.input(screen.getByLabelText('Opis'), { target: { value: 'Opis serii' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj' }));

		expect(mockCreate).toHaveBeenCalledWith({ name: 'X', description: 'Opis serii', kind: 'sea' });
		expect(await screen.findByText('boom')).toBeInTheDocument();
	});

	it('gives an admin both create actions: a series and a standalone leg', async () => {
		await renderPage();
		expect(screen.getByRole('button', { name: '+ Dodaj serię' })).toBeInTheDocument();
		expect(screen.getByRole('link', { name: '+ Nowy rejs' })).toHaveAttribute('href', '/trips/new');
	});

	it('hides both create actions from a member', async () => {
		asMember();
		await renderPage();
		expect(screen.queryByRole('button', { name: '+ Dodaj serię' })).not.toBeInTheDocument();
		expect(screen.queryByRole('link', { name: '+ Nowy rejs' })).not.toBeInTheDocument();
	});

	it.each([
		['series', mockList, 'serie'],
		['standalone legs', mockStandalone, 'rejsy bez serii']
	] as const)(
		'names the failed %s request instead of showing it as empty',
		async (_, mock, label) => {
			mock.mockRejectedValue(new Error('down'));
			await renderPage();
			expect(screen.getByRole('alert')).toHaveTextContent(`Nie udało się wczytać: ${label}`);
		}
	);
});

describe('Rejsy page: Bez serii', () => {
	it('lists standalone legs after the series, each opening its own page', async () => {
		mockList.mockResolvedValue([makeSeries({ id: 4, name: 'May I Sail' })]);
		mockStandalone.mockResolvedValue([
			entry({ kind: 'trip', id: 5, name: 'Chorwacja', start_port: 'Split', end_port: 'Trogir' }),
			entry({ kind: 'voyage', id: 6, name: 'Bałtyk 2019', cruise_name: 'Bałtyk 2019' }),
			entry({ kind: 'cruise', id: 9, name: 'Bez etapów', cruise_id: 9, cruise_name: 'Bez etapów' })
		]);
		await renderPage();

		const group = screen.getByRole('region', { name: 'Bez serii' });
		const seriesList = screen.getByRole('list', { name: 'Serie' });
		expect(
			seriesList.compareDocumentPosition(group) & Node.DOCUMENT_POSITION_FOLLOWING
		).toBeTruthy();

		expect(within(group).getByText('Chorwacja').closest('a')).toHaveAttribute('href', '/trips/5');
		expect(within(group).getByText('Split → Trogir')).toBeInTheDocument();
		expect(within(group).getByText('Bałtyk 2019').closest('a')).toHaveAttribute(
			'href',
			'/voyages/6'
		);
		expect(within(group).getByText('Bałtyk 2019').closest('a')).toHaveTextContent('Zrealizowany');
		const empty = within(group).getByText('Bez etapów').closest('a');
		expect(empty).toHaveAttribute('href', '/cruises/9');
		expect(empty).toHaveTextContent('Brak etapów');
	});

	it("shows a leg's cruise name only when it differs from the leg's", async () => {
		mockStandalone.mockResolvedValue([
			entry({ id: 1, name: 'Etap 1', cruise_name: 'Dawna edycja' }),
			entry({ id: 2, name: 'Solo', cruise_name: 'Solo' })
		]);
		await renderPage();
		const [first, second] = screen.getAllByTestId('standalone-entry');
		expect(first).toHaveTextContent('Dawna edycja');
		expect(second.textContent?.match(/Solo/g)).toHaveLength(1);
	});

	it.each([
		[{ cancelled: true }, 'Anulowany'],
		[{ embark_date: undefined, disembark_date: undefined }, 'Termin do ustalenia'],
		[{ embark_date: '2020-01-01', disembark_date: '2020-01-08' }, 'Do rozliczenia']
	] as const)('describes a trip %j with %s', async (overrides, text) => {
		mockStandalone.mockResolvedValue([entry(overrides)]);
		await renderPage();
		expect(screen.getByTestId('standalone-entry')).toHaveTextContent(text);
	});

	const drafts = [
		entry({ id: 1, name: 'Opublikowany' }),
		entry({ id: 2, name: 'Szkic etapu', published_at: undefined }),
		entry({ id: 3, name: 'Etap szkicu rejsu', cruise_published_at: undefined })
	];

	it.each([
		['an admin', () => {}, [false, true, true]],
		[
			'the captain group',
			() => {
				mockAuth.isAdmin = false;
				mockAuth.user.is_captain = true;
			},
			[false, true, true]
		],
		['a member', asMember, [false, false, false]]
	] as const)('marks hidden drafts for %s', async (_, persona, badges) => {
		persona();
		mockStandalone.mockResolvedValue(drafts);
		await renderPage();
		const cards = screen.getAllByTestId('standalone-entry');
		expect(cards.map((c) => within(c).queryByTestId('hidden-badge') !== null)).toEqual(badges);
	});
});
