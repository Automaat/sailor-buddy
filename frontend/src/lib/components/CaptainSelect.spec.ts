import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/svelte';
import { makeMember, makePerson } from '#lib/test-utils.js';

vi.mock('#lib/api/routes.js', () => ({ listCrew: vi.fn(), listMembers: vi.fn() }));

import { listCrew, listMembers } from '#lib/api/routes.js';
import CaptainSelect from './CaptainSelect.svelte';

const people = listCrew as unknown as ReturnType<typeof vi.fn>;
const members = listMembers as unknown as ReturnType<typeof vi.fn>;

beforeEach(() => {
	people.mockReset().mockResolvedValue([
		makePerson({ id: 10, full_name: 'Zofia Załogantka', user_id: 2, has_account: true }),
		makePerson({ id: 11, full_name: 'Bartek Bez Konta' }),
		makePerson({ id: 12, full_name: 'Anna Kapitan', user_id: 1, has_account: true })
	]);
	members.mockReset().mockResolvedValue([
		makeMember({ id: 1, name: 'Anna Kapitan', is_captain: true }),
		makeMember({ id: 2, name: 'Zofia Załogantka', is_captain: false })
	]);
});

function optionNames(select: HTMLElement) {
	return within(select)
		.getAllByRole('option')
		.map((o) => o.textContent?.trim());
}

describe('CaptainSelect', () => {
	it('lists every person with captain-group accounts first', async () => {
		render(CaptainSelect, { value: 0 });
		await screen.findByRole('option', { name: 'Anna Kapitan' });
		expect(optionNames(screen.getByRole('combobox'))).toEqual([
			'-- Brak --',
			'Anna Kapitan',
			'Bartek Bez Konta',
			'Zofia Załogantka'
		]);
	});

	it('marks captains under their own heading', async () => {
		render(CaptainSelect, { value: 0 });
		const captain = await screen.findByRole('option', { name: 'Anna Kapitan' });
		expect(captain.closest('optgroup')).toHaveAttribute('label', 'Kapitanowie');
		const other = screen.getByRole('option', { name: 'Bartek Bez Konta' });
		expect(other.closest('optgroup')).toHaveAttribute('label', 'Pozostałe osoby');
	});

	it('uses the person id as the value', async () => {
		render(CaptainSelect, { value: 11 });
		await screen.findByRole('option', { name: 'Bartek Bez Konta' });
		expect(screen.getByRole('combobox')).toHaveValue('11');
	});

	it('still lists people when the captain group cannot be loaded', async () => {
		members.mockRejectedValue(new Error('forbidden'));
		vi.spyOn(console, 'error').mockImplementation(() => {});
		render(CaptainSelect, { value: 0 });
		const person = await screen.findByRole('option', { name: 'Anna Kapitan' });
		expect(person.closest('optgroup')).toHaveAttribute('label', 'Pozostałe osoby');
	});

	it('shows the legacy free-text captain until a person is picked', async () => {
		render(CaptainSelect, { value: 0, legacyName: 'Kpt. Spoza' });
		expect(await screen.findByText('Obecnie: Kpt. Spoza (nie wybrano osoby)')).toBeInTheDocument();
	});

	it('hides the legacy name once a person is assigned', async () => {
		render(CaptainSelect, { value: 12, legacyName: 'Kpt. Spoza' });
		await screen.findByRole('option', { name: 'Anna Kapitan' });
		expect(screen.queryByText(/nie wybrano osoby/)).not.toBeInTheDocument();
	});
});
