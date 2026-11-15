import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import { createRawSnippet } from 'svelte';

const h = vi.hoisted(() => ({
	store: {
		user: null as unknown,
		isAuthenticated: true,
		loading: false,
		isAdmin: false,
		firebaseUser: { uid: 'u1' }
	},
	logout: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: { url: new URL('http://localhost/') } }));
vi.mock('#lib/api/routes.js', () => ({ getMe: vi.fn() }));
vi.mock('#lib/stores/linkRequests.svelte.js', () => ({
	linkRequests: { pending: 0, refresh: vi.fn() }
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: {
		get user() {
			return h.store.user;
		},
		set user(u: unknown) {
			h.store.user = u;
		},
		get isAuthenticated() {
			return h.store.isAuthenticated;
		},
		get loading() {
			return h.store.loading;
		},
		get isAdmin() {
			return h.store.isAdmin;
		},
		get firebaseUser() {
			return h.store.firebaseUser;
		},
		logout: h.logout
	}
}));

import { getMe } from '#lib/api/routes.js';
import { ApiError } from '#lib/api/client.js';
import Layout from './+layout.svelte';

const children = createRawSnippet(() => ({ render: () => '<p>club page</p>' }));

beforeEach(() => {
	h.store.user = null;
	h.store.isAdmin = false;
	h.store.isAuthenticated = true;
	h.logout.mockReset().mockResolvedValue(undefined);
	vi.mocked(getMe).mockReset();
});

describe('layout account loading', () => {
	it('explains a suspended account instead of showing the raw API error', async () => {
		vi.mocked(getMe).mockRejectedValue(new ApiError(403, 'account suspended', 'account_suspended'));
		render(Layout, { props: { children } });

		expect(await screen.findByRole('heading', { name: 'Konto zawieszone' })).toBeInTheDocument();
		expect(screen.queryByText('account suspended')).not.toBeInTheDocument();
		expect(screen.queryByText('club page')).not.toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Wyloguj się' }));
		await waitFor(() => expect(h.logout).toHaveBeenCalled());
	});

	it('shows other account loading errors as before', async () => {
		vi.mocked(getMe).mockRejectedValue(new ApiError(500, 'boom'));
		render(Layout, { props: { children } });

		expect(await screen.findByRole('alert')).toHaveTextContent('boom');
		expect(screen.queryByRole('heading', { name: 'Konto zawieszone' })).not.toBeInTheDocument();
	});
});

describe('layout navigation', () => {
	const account = { id: 1, onboarded: true, email_verified: true, suspended: false };

	it.each([
		['a member', false, { ...account, role: 'member', is_captain: false }, false],
		['the captain group', false, { ...account, role: 'member', is_captain: true }, true],
		['an admin', true, { ...account, role: 'admin', is_captain: false }, true]
	])('Jachty item for %s', (_, isAdmin, user, shown) => {
		h.store.isAdmin = isAdmin;
		h.store.user = user;
		render(Layout, { props: { children } });

		const links = screen.queryAllByRole('link', { name: 'Jachty' });
		expect(links.length > 0).toBe(shown);
		expect(screen.getAllByRole('link', { name: 'Rejsy' }).length).toBeGreaterThan(0);
	});
});
