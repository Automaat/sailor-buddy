import { describe, it, expect, vi, beforeEach } from 'vitest';

const { connectAuthEmulator } = vi.hoisted(() => ({ connectAuthEmulator: vi.fn() }));

vi.mock('$env/dynamic/public', () => ({ env: {} }));
vi.mock('firebase/app', () => ({ initializeApp: vi.fn(() => ({})) }));
vi.mock('firebase/auth', () => ({
	getAuth: vi.fn(() => ({})),
	connectAuthEmulator
}));

import { connectEmulator, firebaseAuth } from './firebase';

describe('connectEmulator', () => {
	beforeEach(() => {
		connectAuthEmulator.mockReset();
	});

	it('points the auth instance at the emulator URL', () => {
		connectEmulator(firebaseAuth, 'http://localhost:9099');
		expect(connectAuthEmulator).toHaveBeenCalledWith(firebaseAuth, 'http://localhost:9099');
	});

	it('swallows the error a repeated connect throws', () => {
		connectAuthEmulator.mockImplementation(() => {
			throw new Error('emulator already started');
		});
		expect(() => connectEmulator(firebaseAuth, 'http://localhost:9099')).not.toThrow();
	});
});
