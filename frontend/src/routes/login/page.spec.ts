import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: { url: new URL('http://localhost/login') } }));
vi.mock('#lib/firebase.js', () => ({ firebaseAuth: {} }));
vi.mock('#lib/stores/auth.svelte.js', () => ({ auth: { isAuthenticated: false } }));
vi.mock('firebase/auth', () => ({
	signInWithEmailAndPassword: vi.fn(),
	createUserWithEmailAndPassword: vi.fn(),
	signInWithPopup: vi.fn(),
	updateProfile: vi.fn(),
	GoogleAuthProvider: class {
		setCustomParameters() {}
	}
}));

import LoginPage from './+page.svelte';

describe('login page', () => {
	it('shows the club brand and links to the club site', () => {
		render(LoginPage);
		expect(screen.getByRole('heading', { level: 1, name: 'AKŻ AGH' })).toBeInTheDocument();
		expect(screen.getByRole('img', { name: 'AKŻ AGH' })).toHaveAttribute('src', '/akz-logo.png');
		expect(screen.getByRole('link', { name: 'keja.agh.edu.pl' })).toHaveAttribute(
			'href',
			'https://keja.agh.edu.pl/'
		);
		expect(screen.queryByText(/Kapitanie/)).not.toBeInTheDocument();
		expect(screen.queryByText(/Sailor Buddy/)).not.toBeInTheDocument();
	});

	it('keeps the register subtitle when switching modes', async () => {
		render(LoginPage);
		await fireEvent.click(screen.getByRole('button', { name: 'Zarejestruj' }));
		expect(screen.getByText('Utwórz konto')).toBeInTheDocument();
	});
});
