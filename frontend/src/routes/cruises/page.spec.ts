import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import type { Cruise } from '$lib/api/types';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$lib/api/client', () => ({ api: { list: vi.fn() } }));
vi.mock('$lib/stores/org.svelte', () => ({
	orgStore: {
		isOrgMode: true,
		isOrgAdmin: true,
		currentSlug: 'alfa',
		apiPrefix: () => '/orgs/alfa'
	}
}));

import { api } from '$lib/api/client';
import { orgStore } from '$lib/stores/org.svelte';
import CruisesPage from './+page.svelte';

const apiList = api.list as unknown as ReturnType<typeof vi.fn>;
// orgStore's flags are getter-only on the real store; the mock is a plain
// object, so cast to a mutable view to toggle org context per test.
const mockOrg = orgStore as unknown as { isOrgMode: boolean; isOrgAdmin: boolean };

function makeCruise(overrides: Partial<Cruise> = {}): Cruise {
	return {
		id: 1,
		org_id: 1,
		name: 'Regaty Klubowe',
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

beforeEach(() => {
	apiList.mockReset().mockResolvedValue([]);
	vi.spyOn(console, 'error').mockImplementation(() => {});
	mockOrg.isOrgMode = true;
	mockOrg.isOrgAdmin = true;
});

describe('cruises page — solo mode', () => {
	it('explains cruises are org-only and skips the API call', async () => {
		mockOrg.isOrgMode = false;
		render(CruisesPage);
		expect(
			await screen.findByText('Wydarzenia istnieją tylko w klubie')
		).toBeInTheDocument();
		expect(apiList).not.toHaveBeenCalled();
	});
});

describe('cruises page — org mode', () => {
	it('shows a loading message while cruises are being fetched', () => {
		apiList.mockReturnValue(new Promise(() => {}));
		render(CruisesPage);
		expect(screen.getByText('Wczytywanie...')).toBeInTheDocument();
	});

	it('requests cruises from the org-scoped endpoint', async () => {
		render(CruisesPage);
		await screen.findByText('Brak wydarzeń');
		expect(apiList).toHaveBeenCalledWith('/orgs/alfa/cruises');
	});

	it('shows the empty state when there are no cruises', async () => {
		render(CruisesPage);
		expect(await screen.findByText('Brak wydarzeń')).toBeInTheDocument();
	});

	it('renders cruise cards and marks open enrollment', async () => {
		apiList.mockResolvedValue([
			makeCruise({
				id: 9,
				name: 'Regaty Klubowe',
				start_port: 'Gdynia',
				end_port: 'Sopot',
				max_crew: 12,
				enroll_token: 'abc'
			})
		]);
		render(CruisesPage);

		const card = (await screen.findByText('Regaty Klubowe')).closest('a');
		expect(card).toHaveAttribute('href', '/cruises/9');
		expect(screen.getByText('max 12 os.')).toBeInTheDocument();
		expect(screen.getByText('Zapisy otwarte')).toBeInTheDocument();
	});

	it('shows the create button for an org admin', async () => {
		render(CruisesPage);
		await screen.findByText('Brak wydarzeń');
		expect(screen.getByRole('link', { name: '+ Nowe wydarzenie' })).toBeInTheDocument();
	});

	it('hides the create button for a non-admin member', async () => {
		mockOrg.isOrgAdmin = false;
		render(CruisesPage);
		await screen.findByText('Brak wydarzeń');
		expect(screen.queryByRole('link', { name: '+ Nowe wydarzenie' })).not.toBeInTheDocument();
	});

	it('falls back to the empty state when loading fails', async () => {
		apiList.mockRejectedValue(new Error('network down'));
		render(CruisesPage);
		expect(await screen.findByText('Brak wydarzeń')).toBeInTheDocument();
		expect(console.error).toHaveBeenCalled();
	});
});
