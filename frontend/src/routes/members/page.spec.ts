import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { makeMember } from '#lib/test-utils.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('#lib/api/routes.js', () => ({
	listMembers: vi.fn(),
	updateMemberRole: vi.fn(),
	updateMemberCaptain: vi.fn(),
	updateMemberMembership: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));

import {
	listMembers,
	updateMemberRole,
	updateMemberCaptain,
	updateMemberMembership
} from '#lib/api/routes.js';
import MembersPage from './+page.svelte';

const listMock = listMembers as unknown as ReturnType<typeof vi.fn>;
const roleMock = updateMemberRole as unknown as ReturnType<typeof vi.fn>;
const captainMock = updateMemberCaptain as unknown as ReturnType<typeof vi.fn>;
const membershipMock = updateMemberMembership as unknown as ReturnType<typeof vi.fn>;

function roster() {
	return [
		makeMember({ id: 1, name: 'Kasia Admin', email: 'kasia@agh.dev', role: 'admin' }),
		makeMember({ id: 2, name: 'Olek Admin', email: 'olek@agh.dev', role: 'admin' }),
		makeMember({
			id: 3,
			name: 'Piotr Kapitan',
			email: 'piotr@agh.dev',
			role: 'member',
			patent_number: 'KJ-1',
			is_captain: true
		}),
		makeMember({
			id: 4,
			name: 'Ewa Sternik',
			email: 'ewa@x.dev',
			role: 'member',
			patent_number: 'JS-2'
		}),
		makeMember({
			id: 5,
			name: 'Zofia Sympatyk',
			email: 'zofia@x.dev',
			role: 'member',
			membership: 'sympatyk',
			patent_number: 'JS-3'
		}),
		makeMember({
			id: 6,
			name: 'Gość Nowy',
			email: 'gosc@x.dev',
			role: 'member',
			membership: 'gosc',
			onboarded: false
		})
	];
}

async function renderPage() {
	render(MembersPage);
	await screen.findByText('Kasia Admin');
}

function rowNames() {
	return screen
		.getAllByRole('row')
		.slice(1)
		.map((r) => within(r).getAllByRole('cell')[1].textContent?.trim())
		.map((email) => roster().find((m) => m.email === email)?.name);
}

beforeEach(() => {
	listMock.mockReset().mockResolvedValue(roster());
	roleMock.mockReset().mockResolvedValue(undefined);
	captainMock.mockReset().mockResolvedValue(undefined);
	membershipMock.mockReset().mockResolvedValue(undefined);
});

afterEach(() => {
	vi.restoreAllMocks();
	vi.useRealTimers();
});

describe('members page: search and filters', () => {
	it('filters by name or e-mail as the admin types', async () => {
		await renderPage();
		const search = screen.getByRole('searchbox', { name: 'Szukaj członków' });

		await fireEvent.input(search, { target: { value: 'kapitan' } });
		expect(rowNames()).toEqual(['Piotr Kapitan']);

		await fireEvent.input(search, { target: { value: 'X.DEV' } });
		expect(rowNames()).toEqual(['Ewa Sternik', 'Zofia Sympatyk', 'Gość Nowy']);

		await fireEvent.input(search, { target: { value: 'gosc nowy' } });
		expect(rowNames()).toEqual(['Gość Nowy']);

		await fireEvent.input(search, { target: { value: 'nikt' } });
		expect(screen.getByText('Brak osób spełniających kryteria')).toBeInTheDocument();
	});

	it('shows filter chips with counts', async () => {
		await renderPage();
		const chips = within(screen.getByRole('group', { name: 'Filtr członków' }));
		expect(chips.getByRole('button', { name: 'Wszyscy (6)' })).toHaveAttribute(
			'aria-pressed',
			'true'
		);
		expect(chips.getByRole('button', { name: 'Członek (4)' })).toBeInTheDocument();
		expect(chips.getByRole('button', { name: 'Sympatyk (1)' })).toBeInTheDocument();
		expect(chips.getByRole('button', { name: 'Gość (1)' })).toBeInTheDocument();
		expect(chips.getByRole('button', { name: 'Kapitanowie (1)' })).toBeInTheDocument();
		expect(chips.getByRole('button', { name: 'Profil niekompletny (1)' })).toBeInTheDocument();
	});

	it.each([
		['Sympatyk (1)', ['Zofia Sympatyk']],
		['Gość (1)', ['Gość Nowy']],
		['Kapitanowie (1)', ['Piotr Kapitan']],
		['Profil niekompletny (1)', ['Gość Nowy']],
		['Członek (4)', ['Kasia Admin', 'Olek Admin', 'Piotr Kapitan', 'Ewa Sternik']]
	])('chip %s narrows the list', async (chip, want) => {
		await renderPage();
		const button = screen.getByRole('button', { name: chip });
		await fireEvent.click(button);
		expect(button).toHaveAttribute('aria-pressed', 'true');
		expect(rowNames()).toEqual(want);
	});

	it('combines a chip with the search query', async () => {
		await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: 'Członek (4)' }));
		await fireEvent.input(screen.getByRole('searchbox'), { target: { value: 'admin' } });
		expect(rowNames()).toEqual(['Kasia Admin', 'Olek Admin']);
	});
});

describe('members page: captain cell', () => {
	it('shows a checkbox only for members with a patent number', async () => {
		await renderPage();
		expect(screen.getByRole('checkbox', { name: 'Kapitan: Piotr Kapitan' })).toBeChecked();
		expect(screen.getByRole('checkbox', { name: 'Kapitan: Ewa Sternik' })).toBeEnabled();
		expect(
			screen.queryByRole('checkbox', { name: 'Kapitan: Kasia Admin' })
		).not.toBeInTheDocument();
		expect(screen.queryByRole('checkbox', { name: 'Kapitan: Gość Nowy' })).not.toBeInTheDocument();
		expect(screen.queryByText('Brak numeru patentu na koncie')).not.toBeInTheDocument();
	});

	it('explains a missing patent in a tooltip on a single dash', async () => {
		await renderPage();
		const row = screen.getByText('kasia@agh.dev').closest('tr') as HTMLElement;
		expect(within(row).getByText('—')).toHaveAttribute('title', 'Brak numeru patentu na koncie');
	});

	it('disables the checkbox for a non-member with a patent and says why', async () => {
		await renderPage();
		const box = screen.getByRole('checkbox', { name: 'Kapitan: Zofia Sympatyk' });
		expect(box).toBeDisabled();
		expect(box.closest('label')).toHaveAttribute(
			'title',
			'Kapitanem może być tylko członek klubu'
		);
	});
});

describe('members page: role control', () => {
	it('grants admin rights from the role select without confirmation', async () => {
		const confirmSpy = vi.spyOn(window, 'confirm');
		await renderPage();
		await fireEvent.change(screen.getByRole('combobox', { name: 'Rola: Ewa Sternik' }), {
			target: { value: 'admin' }
		});
		await waitFor(() => expect(roleMock).toHaveBeenCalledWith(4, 'admin'));
		expect(confirmSpy).not.toHaveBeenCalled();
		expect(await screen.findByRole('status')).toHaveTextContent(
			'Zapisano rolę: Ewa Sternik — Administrator'
		);
	});

	it('asks for confirmation before removing admin rights', async () => {
		const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
		await renderPage();
		await fireEvent.change(screen.getByRole('combobox', { name: 'Rola: Olek Admin' }), {
			target: { value: 'member' }
		});
		expect(confirmSpy).toHaveBeenCalledWith('Odebrać uprawnienia administratora: Olek Admin?');
		await waitFor(() => expect(roleMock).toHaveBeenCalledWith(2, 'member'));
	});

	it('keeps the admin role when the confirmation is cancelled', async () => {
		vi.spyOn(window, 'confirm').mockReturnValue(false);
		await renderPage();
		const select = screen.getByRole('combobox', {
			name: 'Rola: Olek Admin'
		}) as HTMLSelectElement;
		await fireEvent.change(select, { target: { value: 'member' } });
		expect(roleMock).not.toHaveBeenCalled();
		expect(select.value).toBe('admin');
	});

	it('locks the role of the last admin', async () => {
		listMock.mockResolvedValue(roster().map((m) => (m.id === 2 ? { ...m, role: 'member' } : m)));
		await renderPage();
		const select = screen.getByRole('combobox', { name: 'Rola: Kasia Admin' });
		expect(select).toBeDisabled();
		expect(select).toHaveAttribute('title', 'Klub musi mieć co najmniej jednego administratora');
	});
});

describe('members page: save feedback', () => {
	it('confirms a saved membership change and hides the message after a while', async () => {
		await renderPage();
		vi.useFakeTimers();
		await fireEvent.change(screen.getByRole('combobox', { name: 'Członkostwo: Gość Nowy' }), {
			target: { value: 'sympatyk' }
		});
		await vi.waitFor(() => expect(membershipMock).toHaveBeenCalledWith(6, 'sympatyk'));
		await vi.waitFor(() =>
			expect(screen.getByRole('status')).toHaveTextContent(
				'Zapisano członkostwo: Gość Nowy — Sympatyk'
			)
		);
		await vi.advanceTimersByTimeAsync(3500);
		expect(screen.queryByRole('status')).not.toBeInTheDocument();
	});

	it('reports a failed membership change and restores the select', async () => {
		membershipMock.mockRejectedValue(new Error('Serwer odrzucił zmianę'));
		await renderPage();
		const select = screen.getByRole('combobox', {
			name: 'Członkostwo: Gość Nowy'
		}) as HTMLSelectElement;
		await fireEvent.change(select, { target: { value: 'czlonek' } });
		expect(await screen.findByRole('alert')).toHaveTextContent('Serwer odrzucił zmianę');
		expect(select.value).toBe('gosc');
		expect(screen.getByRole('button', { name: 'Gość (1)' })).toBeInTheDocument();
	});

	it('confirms a saved captain change', async () => {
		await renderPage();
		await fireEvent.click(screen.getByRole('checkbox', { name: 'Kapitan: Ewa Sternik' }));
		await waitFor(() => expect(captainMock).toHaveBeenCalledWith(4, true));
		expect(await screen.findByRole('status')).toHaveTextContent('Dodano do kadry: Ewa Sternik');
		expect(screen.getByRole('button', { name: 'Kapitanowie (2)' })).toBeInTheDocument();
	});

	it('reports a failed captain change and unticks the box', async () => {
		captainMock.mockRejectedValue(new Error(''));
		await renderPage();
		const box = screen.getByRole('checkbox', { name: 'Kapitan: Ewa Sternik' });
		await fireEvent.click(box);
		expect(await screen.findByRole('alert')).toHaveTextContent(
			'Nie udało się zmienić statusu kapitana'
		);
		expect(box).not.toBeChecked();
	});

	it('reports a failed role change and restores the select', async () => {
		roleMock.mockRejectedValue(new Error('Brak uprawnień'));
		await renderPage();
		const select = screen.getByRole('combobox', {
			name: 'Rola: Ewa Sternik'
		}) as HTMLSelectElement;
		await fireEvent.change(select, { target: { value: 'admin' } });
		expect(await screen.findByRole('alert')).toHaveTextContent('Brak uprawnień');
		expect(select.value).toBe('member');
	});
});
