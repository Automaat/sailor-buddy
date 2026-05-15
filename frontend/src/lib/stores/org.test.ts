import { describe, it, expect, beforeEach, vi } from 'vitest';
import type { Organization } from '$lib/api/types';

vi.mock('$lib/api/client', () => ({
	api: { get: vi.fn() }
}));

import { api } from '$lib/api/client';
import { orgStore } from './org.svelte';

const mockGet = api.get as unknown as ReturnType<typeof vi.fn>;

function org(slug: string, role: string): Organization {
	return {
		id: 1,
		name: slug,
		slug,
		role,
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z'
	};
}

beforeEach(() => {
	mockGet.mockReset();
	localStorage.clear();
	orgStore.clear();
});

describe('orgStore.refresh', () => {
	it('auto-selects the first org when none is stored', async () => {
		mockGet.mockResolvedValue([org('alfa', 'admin'), org('beta', 'crew')]);
		await orgStore.refresh();
		expect(orgStore.currentSlug).toBe('alfa');
		expect(orgStore.isOrgMode).toBe(true);
		expect(localStorage.getItem('sailor-buddy-org')).toBe('alfa');
	});

	it('keeps a still-valid stored org selection', async () => {
		orgStore.select('beta');
		mockGet.mockResolvedValue([org('alfa', 'admin'), org('beta', 'crew')]);
		await orgStore.refresh();
		expect(orgStore.currentSlug).toBe('beta');
	});

	it('falls back to the first org when the stored selection is gone', async () => {
		orgStore.select('zeta');
		mockGet.mockResolvedValue([org('alfa', 'admin')]);
		await orgStore.refresh();
		expect(orgStore.currentSlug).toBe('alfa');
	});

	it('stays org-less when the user belongs to no org', async () => {
		mockGet.mockResolvedValue([]);
		await orgStore.refresh();
		expect(orgStore.currentSlug).toBeNull();
		expect(orgStore.isOrgMode).toBe(false);
		expect(localStorage.getItem('sailor-buddy-org')).toBeNull();
	});

	it('marks the store loaded after a successful refresh', async () => {
		expect(orgStore.loaded).toBe(false);
		mockGet.mockResolvedValue([org('alfa', 'admin')]);
		await orgStore.refresh();
		expect(orgStore.loaded).toBe(true);
	});
});

describe('orgStore.isOrgAdmin', () => {
	it('is true when the current org role is admin', async () => {
		mockGet.mockResolvedValue([org('alfa', 'admin')]);
		await orgStore.refresh();
		expect(orgStore.isOrgAdmin).toBe(true);
	});

	it('is false for a non-admin member', async () => {
		mockGet.mockResolvedValue([org('alfa', 'crew')]);
		await orgStore.refresh();
		expect(orgStore.isOrgAdmin).toBe(false);
	});

	it('is false when the user belongs to no org', async () => {
		mockGet.mockResolvedValue([]);
		await orgStore.refresh();
		expect(orgStore.isOrgAdmin).toBe(false);
	});
});

describe('orgStore.canSwitch', () => {
	it('is false with a single org', async () => {
		mockGet.mockResolvedValue([org('alfa', 'admin')]);
		await orgStore.refresh();
		expect(orgStore.canSwitch).toBe(false);
	});

	it('is false for a multi-org non-admin', async () => {
		mockGet.mockResolvedValue([org('alfa', 'crew'), org('beta', 'captain')]);
		await orgStore.refresh();
		expect(orgStore.canSwitch).toBe(false);
	});

	it('is true for a multi-org admin', async () => {
		mockGet.mockResolvedValue([org('alfa', 'crew'), org('beta', 'admin')]);
		await orgStore.refresh();
		expect(orgStore.canSwitch).toBe(true);
	});
});

describe('orgStore selection helpers', () => {
	it('apiPrefix reflects the selected org', async () => {
		mockGet.mockResolvedValue([org('alfa', 'admin')]);
		await orgStore.refresh();
		expect(orgStore.apiPrefix()).toBe('/orgs/alfa');
	});

	it('apiPrefix is empty without an org', () => {
		expect(orgStore.apiPrefix()).toBe('');
	});

	it('clear() resets orgs, selection and storage', async () => {
		mockGet.mockResolvedValue([org('alfa', 'admin')]);
		await orgStore.refresh();
		orgStore.clear();
		expect(orgStore.orgs).toEqual([]);
		expect(orgStore.currentSlug).toBeNull();
		expect(orgStore.loaded).toBe(false);
		expect(localStorage.getItem('sailor-buddy-org')).toBeNull();
	});
});
