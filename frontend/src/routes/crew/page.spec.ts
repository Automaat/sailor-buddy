import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { makePerson } from '#lib/test-utils.js';

vi.mock('#lib/api/routes.js', () => ({
	listCrew: vi.fn(),
	createCrew: vi.fn(),
	listPersonLinkRequests: vi.fn(),
	approvePersonLinkRequest: vi.fn(),
	rejectPersonLinkRequest: vi.fn(),
	countPersonLinkRequests: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));

import {
	listCrew,
	createCrew,
	listPersonLinkRequests,
	approvePersonLinkRequest,
	rejectPersonLinkRequest
} from '#lib/api/routes.js';
import { ApiError } from '#lib/api/client.js';
import { linkRequests } from '#lib/stores/linkRequests.svelte.js';
import PeoplePage from './+page.svelte';

const listMock = vi.mocked(listCrew);
const createMock = vi.mocked(createCrew);
const requestsMock = vi.mocked(listPersonLinkRequests);
const approveMock = vi.mocked(approvePersonLinkRequest);
const rejectMock = vi.mocked(rejectPersonLinkRequest);

function pendingRequest() {
	return {
		id: 9,
		user_id: 5,
		user_name: 'Ola Nowa',
		user_email: 'ola@example.dev',
		person_id: 3,
		person_name: 'Zofia Łęcka',
		status: 'pending' as const,
		created_at: '2026-01-01T00:00:00Z'
	};
}

function people() {
	return [
		makePerson({
			id: 1,
			full_name: 'Anna Nowak',
			other_names: ['Anna Kowalska'],
			email: 'anna@example.dev',
			has_account: true,
			user_id: 5
		}),
		makePerson({ id: 2, full_name: 'Łukasz Wiśniewski', other_names: ['Lukas Wisniewski'] }),
		makePerson({ id: 3, full_name: 'Zofia Łęcka', email: 'zosia@example.dev' })
	];
}

function names() {
	return screen.queryAllByTestId('person').map((row) => row.querySelector('span')?.textContent);
}

async function renderPage() {
	render(PeoplePage);
	await screen.findByText('Anna Nowak');
}

beforeEach(() => {
	listMock.mockReset().mockResolvedValue(people());
	createMock.mockReset();
	requestsMock.mockReset().mockResolvedValue([]);
	approveMock.mockReset();
	rejectMock.mockReset();
	linkRequests.pending = 0;
});

describe('people page', () => {
	it('is titled Osoby and marks who has an account', async () => {
		await renderPage();
		expect(screen.getByRole('heading', { name: 'Osoby' })).toBeInTheDocument();
		const rows = screen.getAllByTestId('person');
		expect(within(rows[0]).getByText('ma konto')).toBeInTheDocument();
		expect(within(rows[1]).getByText('bez konta')).toBeInTheDocument();
		expect(within(rows[2]).getByText('bez konta')).toBeInTheDocument();
	});

	it.each([
		['nowak', ['Anna Nowak']],
		['kowalska', ['Anna Nowak']],
		['lukasz', ['Łukasz Wiśniewski']],
		['WISNIEWSKI', ['Łukasz Wiśniewski']],
		['zosia@', ['Zofia Łęcka']],
		['example.dev', ['Anna Nowak', 'Zofia Łęcka']],
		['nikt', []]
	])('search %s matches names, other names and e-mail', async (query, want) => {
		await renderPage();
		await fireEvent.input(screen.getByRole('searchbox', { name: 'Szukaj osób' }), {
			target: { value: query }
		});
		expect(names()).toEqual(want);
		if (want.length === 0) {
			expect(screen.getByText('Nikt nie pasuje do wyszukiwania')).toBeInTheDocument();
		}
	});

	it('filters by account', async () => {
		await renderPage();
		const group = within(screen.getByRole('group', { name: 'Konto' }));
		await fireEvent.click(group.getByRole('button', { name: 'Z kontem' }));
		expect(names()).toEqual(['Anna Nowak']);
		await fireEvent.click(group.getByRole('button', { name: 'Bez konta' }));
		expect(names()).toEqual(['Łukasz Wiśniewski', 'Zofia Łęcka']);
	});

	it('adds a person without an account with other names', async () => {
		createMock.mockResolvedValue(
			makePerson({ id: 9, full_name: 'Ewa Maj', other_names: ['Ewa Kos', 'Ewa Majówna'] })
		);
		await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: '+ Dodaj osobę' }));
		await fireEvent.input(screen.getByLabelText('Imię i nazwisko *'), {
			target: { value: 'Ewa Maj' }
		});
		await fireEvent.input(screen.getByLabelText('Inne nazwiska / pisownie'), {
			target: { value: ' Ewa Kos, , Ewa Majówna ' }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj' }));

		await waitFor(() =>
			expect(createMock).toHaveBeenCalledWith({
				full_name: 'Ewa Maj',
				other_names: ['Ewa Kos', 'Ewa Majówna'],
				email: undefined,
				patent_number: undefined
			})
		);
		await screen.findByText('Ewa Maj');
		await fireEvent.input(screen.getByRole('searchbox', { name: 'Szukaj osób' }), {
			target: { value: 'majowna' }
		});
		expect(names()).toEqual(['Ewa Maj']);
	});
});

describe('people page "To ja" requests', () => {
	it('lists pending requests, approves one and refreshes the people and the nav count', async () => {
		requestsMock.mockResolvedValueOnce([pendingRequest()]).mockResolvedValueOnce([]);
		approveMock.mockResolvedValue({ ...pendingRequest(), status: 'approved' });
		await renderPage();
		const row = await screen.findByTestId('link-request');
		expect(row.textContent).toContain('Ola Nowa');
		expect(row.textContent).toContain('Zofia Łęcka');
		expect(linkRequests.pending).toBe(1);

		await fireEvent.click(screen.getByRole('button', { name: 'Zatwierdź: Ola Nowa → Zofia Łęcka' }));

		await waitFor(() => expect(approveMock).toHaveBeenCalledWith(9));
		expect(await screen.findByText('Połączono konto Ola Nowa z osobą Zofia Łęcka.')).toBeTruthy();
		await waitFor(() => expect(listMock).toHaveBeenCalledTimes(2));
		expect(screen.queryByTestId('link-request')).toBeNull();
		expect(linkRequests.pending).toBe(0);
	});

	it('rejects a request', async () => {
		requestsMock.mockResolvedValueOnce([pendingRequest()]).mockResolvedValueOnce([]);
		rejectMock.mockResolvedValue({ ...pendingRequest(), status: 'rejected' });
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Odrzuć: Ola Nowa → Zofia Łęcka' }));
		await waitFor(() => expect(rejectMock).toHaveBeenCalledWith(9));
		expect(approveMock).not.toHaveBeenCalled();
	});

	it('explains a stale request', async () => {
		requestsMock.mockResolvedValue([pendingRequest()]);
		approveMock.mockRejectedValue(new ApiError(409, 'stale'));
		await renderPage();
		await fireEvent.click(await screen.findByRole('button', { name: 'Zatwierdź: Ola Nowa → Zofia Łęcka' }));
		expect(await screen.findByText(/Prośba jest nieaktualna/)).toBeTruthy();
	});

	it('shows nothing when no request waits', async () => {
		await renderPage();
		await waitFor(() => expect(requestsMock).toHaveBeenCalled());
		expect(screen.queryByText('Prośby „To ja”')).toBeNull();
	});
});
