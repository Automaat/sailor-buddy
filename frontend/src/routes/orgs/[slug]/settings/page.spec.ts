import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import { makeOrg } from '$lib/test-utils';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: { params: { slug: 'alfa' } } }));
vi.mock('$lib/api/client', () => ({
	api: { get: vi.fn(), put: vi.fn(), del: vi.fn() }
}));
vi.mock('$lib/stores/org.svelte', () => ({
	orgStore: {
		loaded: true,
		orgs: [{ slug: 'alfa', role: 'admin' }],
		refresh: vi.fn().mockResolvedValue(undefined),
		select: vi.fn()
	}
}));

import { goto } from '$app/navigation';
import { api } from '$lib/api/client';
import { orgStore } from '$lib/stores/org.svelte';
import SettingsPage from './+page.svelte';

const apiGet = api.get as unknown as ReturnType<typeof vi.fn>;
const apiPut = api.put as unknown as ReturnType<typeof vi.fn>;
const apiDel = api.del as unknown as ReturnType<typeof vi.fn>;

beforeEach(() => {
	apiGet.mockReset().mockResolvedValue(makeOrg());
	apiPut.mockReset().mockResolvedValue(undefined);
	apiDel.mockReset().mockResolvedValue(undefined);
	(orgStore.refresh as ReturnType<typeof vi.fn>).mockClear();
	(orgStore.select as ReturnType<typeof vi.fn>).mockClear();
	(goto as ReturnType<typeof vi.fn>).mockClear();
	window.confirm = vi.fn(() => true);
});

describe('org settings page', () => {
	it('loads the org and populates the form', async () => {
		render(SettingsPage);
		const name = (await screen.findByLabelText('Nazwa *')) as HTMLInputElement;
		expect(name.value).toBe('Klub Alfa');
		expect(apiGet).toHaveBeenCalledWith('/orgs/alfa');
	});

	it('saves edited fields and confirms success', async () => {
		render(SettingsPage);
		const name = (await screen.findByLabelText('Nazwa *')) as HTMLInputElement;
		await fireEvent.input(name, { target: { value: 'Klub Beta' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz' }));

		await waitFor(() =>
			expect(apiPut).toHaveBeenCalledWith('/orgs/alfa', expect.objectContaining({ name: 'Klub Beta' }))
		);
		expect(await screen.findByText('Zapisano')).toBeInTheDocument();
		expect(orgStore.refresh).toHaveBeenCalled();
	});

	it('shows an error when saving fails', async () => {
		apiPut.mockRejectedValue(new Error('Brak uprawnień'));
		render(SettingsPage);
		await screen.findByLabelText('Nazwa *');
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz' }));

		expect((await screen.findAllByText('Brak uprawnień')).length).toBeGreaterThan(0);
	});

	it('shows an error when the org fails to load', async () => {
		apiGet.mockRejectedValue(new Error('Nie znaleziono klubu'));
		render(SettingsPage);
		expect(await screen.findByText('Nie znaleziono klubu')).toBeInTheDocument();
	});

	it('deletes the org after confirmation', async () => {
		render(SettingsPage);
		await screen.findByLabelText('Nazwa *');
		await fireEvent.click(screen.getByRole('button', { name: 'Usuń klub' }));

		await waitFor(() => expect(apiDel).toHaveBeenCalledWith('/orgs/alfa'));
		expect(orgStore.select).toHaveBeenCalledWith(null);
		expect(goto).toHaveBeenCalledWith('/orgs');
	});

	it('does not delete the org when confirmation is dismissed', async () => {
		window.confirm = vi.fn(() => false);
		render(SettingsPage);
		await screen.findByLabelText('Nazwa *');
		await fireEvent.click(screen.getByRole('button', { name: 'Usuń klub' }));

		expect(apiDel).not.toHaveBeenCalled();
	});

	it('shows an error when deletion fails', async () => {
		apiDel.mockRejectedValue(new Error('Nie można usunąć'));
		render(SettingsPage);
		await screen.findByLabelText('Nazwa *');
		await fireEvent.click(screen.getByRole('button', { name: 'Usuń klub' }));

		expect((await screen.findAllByText('Nie można usunąć')).length).toBeGreaterThan(0);
		expect(goto).not.toHaveBeenCalled();
	});

	it('renders empty inputs when optional fields are absent', async () => {
		apiGet.mockResolvedValue(
			makeOrg({ description: undefined, city: undefined, website: undefined, pzz_club_number: undefined })
		);
		render(SettingsPage);
		const city = (await screen.findByLabelText('Miasto')) as HTMLInputElement;
		expect(city.value).toBe('');
	});
});
