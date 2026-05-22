import { describe, it, expect, vi } from 'vitest';

// Capture the firebase auth-state callback so the test can drive sign-in and
// sign-out transitions; stub signOut so logout resolves without a real client.
const onAuthStateChanged = vi.hoisted(() => vi.fn());
const signOut = vi.hoisted(() => vi.fn().mockResolvedValue(undefined));

vi.mock('firebase/auth', () => ({ onAuthStateChanged }));
vi.mock('$lib/firebase', () => ({ firebaseAuth: { signOut } }));

import { auth } from './auth.svelte';
import type { User } from '$lib/api/types';

// authCallback is the listener auth.svelte.ts registered at module load.
const authCallback = onAuthStateChanged.mock.calls[0][1] as (u: unknown) => void;

const fbUser = { getIdToken: vi.fn().mockResolvedValue('id-token') };

describe('auth store lifecycle', () => {
	it('starts unauthenticated and loading', () => {
		expect(auth.loading).toBe(true);
		expect(auth.isAuthenticated).toBe(false);
		expect(auth.user).toBeNull();
		expect(auth.firebaseUser).toBeNull();
		expect(auth.isAdmin).toBe(false);
	});

	it('returns a null id token while signed out', async () => {
		await expect(auth.getIdToken()).resolves.toBeNull();
	});

	it('reflects a sign-in pushed through the firebase listener', async () => {
		authCallback(fbUser);
		expect(auth.loading).toBe(false);
		expect(auth.isAuthenticated).toBe(true);
		expect(auth.firebaseUser).toEqual(fbUser);
		await expect(auth.getIdToken()).resolves.toBe('id-token');
	});

	it('exposes the db profile and derives isAdmin from its role', () => {
		auth.user = { id: 1, name: 'Ann', role: 'admin' } as User;
		expect(auth.user?.name).toBe('Ann');
		expect(auth.isAdmin).toBe(true);

		auth.user = { id: 2, name: 'Bo', role: 'member' } as User;
		expect(auth.isAdmin).toBe(false);
	});

	it('clears the db profile when the listener reports a sign-out', () => {
		authCallback(null);
		expect(auth.user).toBeNull();
		expect(auth.isAuthenticated).toBe(false);
	});

	it('logout signs out of firebase and clears state', async () => {
		authCallback(fbUser);
		auth.user = { id: 1, name: 'Ann', role: 'admin' } as User;

		await auth.logout();

		expect(signOut).toHaveBeenCalled();
		expect(auth.firebaseUser).toBeNull();
		expect(auth.user).toBeNull();
		expect(auth.isAuthenticated).toBe(false);
	});
});
