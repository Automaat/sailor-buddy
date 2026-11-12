import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import { makeMember } from '#lib/test-utils.js';

vi.mock('#lib/api/routes.js', () => ({ listMembers: vi.fn() }));

import { listMembers } from '#lib/api/routes.js';
import CaptainSelect from './CaptainSelect.svelte';

const list = listMembers as unknown as ReturnType<typeof vi.fn>;

beforeEach(() => {
	list.mockReset().mockResolvedValue([
		makeMember({ id: 1, name: 'Anna Kapitan', is_captain: true }),
		makeMember({ id: 2, name: 'Jan Załogant', is_captain: false })
	]);
});

describe('CaptainSelect', () => {
	it('offers only members of the captain group', async () => {
		render(CaptainSelect, { value: 0 });
		expect(await screen.findByRole('option', { name: 'Anna Kapitan' })).toBeInTheDocument();
		expect(screen.queryByRole('option', { name: 'Jan Załogant' })).not.toBeInTheDocument();
	});

	it('shows the legacy free-text captain until one is picked', async () => {
		render(CaptainSelect, { value: 0, legacyName: 'Kpt. Spoza' });
		expect(await screen.findByText('Obecnie: Kpt. Spoza (spoza kadry)')).toBeInTheDocument();
	});

	it('hides the legacy name once a captain is assigned', async () => {
		render(CaptainSelect, { value: 1, legacyName: 'Kpt. Spoza' });
		await screen.findByRole('option', { name: 'Anna Kapitan' });
		expect(screen.queryByText(/spoza kadry/)).not.toBeInTheDocument();
	});
});
