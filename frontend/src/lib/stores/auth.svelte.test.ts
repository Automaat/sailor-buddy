import { describe, it, expect, vi } from 'vitest';

// Capture the firebase auth-state callback so the test can drive sign-in and
// sign-out transitions; stub signOut so logout resolves without a real client.
const onAuthStateChanged = vi.hoisted(() => vi.fn());
const signOut = vi.hoisted(() => vi.fn().mockResolvedValue(undefined));

vi.mock('firebase/auth', () => ({ onAuthStateChanged }));
vi.mock('$lib/firebase', () => ({ firebaseAuth: { signOut } }));

import { auth } from './auth.svelte';
import type { User } from '$lib/api/types';

const fbUser = { getIdToken: vi.fn().mockResolvedValue('id-token') };

// The store is a module singleton that registers one firebase listener at
// import and mutates shared state across its lifetime, so the lifecycle is
// covered as a single ordered walk rather than as isolated cases.
describe('auth store lifecycle', () => {
	it('registers a firebase auth-state listener at import', () => {
		expect(onAuthStateChanged).toHaveBeenCalledTimes(1);
		expect(typeof onAuthStateChanged.mock.calls[0][1]).toBe('function');
	});

	it('walks sign-in, profile load and sign-out', async () => {
		// authListener is the callback the store handed to onAuthStateChanged.
		const authListener = onAuthStateChanged.mock.calls[0][1] as (u: unknown) => void;

		// Initial state: loading, unauthenticated, no token.
		expect(auth.loading).toBe(true);
		expect(auth.isAuthenticated).toBe(false);
		expect(auth.user).toBeNull();
		expect(auth.firebaseUser).toBeNull();
		expect(auth.isAdmin).toBe(false);
		await expect(auth.getIdToken()).resolves.toBeNull();

		// Sign-in pushed through the listener clears loading and exposes the user.
		authListener(fbUser);
		expect(auth.loading).toBe(false);
		expect(auth.isAuthenticated).toBe(true);
		expect(auth.firebaseUser).toEqual(fbUser);
		await expect(auth.getIdToken()).resolves.toBe('id-token');

		// The db profile is set separately and drives isAdmin.
		auth.user = { id: 1, name: 'Ann', role: 'admin' } as User;
		expect(auth.user?.name).toBe('Ann');
		expect(auth.isAdmin).toBe(true);
		auth.user = { id: 2, name: 'Bo', role: 'member' } as User;
		expect(auth.isAdmin).toBe(false);

		// A sign-out reported by the listener clears the db profile.
		authListener(null);
		expect(auth.user).toBeNull();
		expect(auth.isAuthenticated).toBe(false);

		// logout signs out of firebase and clears both users.
		authListener(fbUser);
		auth.user = { id: 1, name: 'Ann', role: 'admin' } as User;
		await auth.logout();
		expect(signOut).toHaveBeenCalled();
		expect(auth.firebaseUser).toBeNull();
		expect(auth.user).toBeNull();
		expect(auth.isAuthenticated).toBe(false);
	});
});
