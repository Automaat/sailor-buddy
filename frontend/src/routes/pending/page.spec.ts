import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

const h = vi.hoisted(() => ({
	store: { user: null as unknown },
	fbUser: {
		uid: 'u1',
		reload: vi.fn(),
		getIdToken: vi.fn()
	},
	logout: vi.fn()
}));

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('firebase/auth', () => ({ sendEmailVerification: vi.fn() }));
vi.mock('#lib/firebase.js', () => ({
	firebaseAuth: {
		get currentUser() {
			return h.fbUser;
		}
	}
}));
vi.mock('#lib/api/routes.js', () => ({ getMe: vi.fn() }));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: {
		get user() {
			return h.store.user;
		},
		set user(u: unknown) {
			h.store.user = u;
		},
		logout: h.logout
	}
}));

import { goto } from '$app/navigation';
import { sendEmailVerification } from 'firebase/auth';
import { getMe } from '#lib/api/routes.js';
import PendingPage from './+page.svelte';

function user(overrides: Record<string, unknown> = {}) {
	return {
		id: 1,
		email: 'jan@example.com',
		email_verified: false,
		suspended: false,
		onboarded: true,
		...overrides
	};
}

beforeEach(() => {
	vi.mocked(goto).mockReset();
	vi.mocked(sendEmailVerification).mockReset().mockResolvedValue(undefined);
	vi.mocked(getMe).mockReset();
	h.fbUser.reload.mockReset().mockResolvedValue(undefined);
	h.fbUser.getIdToken.mockReset().mockResolvedValue('token');
	h.logout.mockReset().mockResolvedValue(undefined);
	h.store.user = user();
});

describe('access page', () => {
	it('asks only for email verification, never for approval', () => {
		render(PendingPage);
		expect(screen.getByRole('heading', { name: 'Potwierdź adres e-mail' })).toBeVisible();
		expect(screen.getByText('jan@example.com')).toBeVisible();
		expect(screen.queryByText(/zatwierdz/i)).toBeNull();
		expect(screen.getByRole('link', { name: 'Mój profil' })).toHaveAttribute('href', '/profile');
	});

	it('sends the verification link', async () => {
		render(PendingPage);
		await fireEvent.click(screen.getByRole('button', { name: 'Wyślij link weryfikacyjny' }));
		await waitFor(() => expect(sendEmailVerification).toHaveBeenCalledWith(h.fbUser));
		expect(await screen.findByRole('status')).toHaveTextContent(
			'Wysłano link do potwierdzenia adresu e-mail.'
		);
	});

	it('opens the app once the refreshed profile has a verified email', async () => {
		vi.mocked(getMe).mockResolvedValue(user({ email_verified: true }) as never);
		render(PendingPage);
		await fireEvent.click(screen.getByRole('button', { name: 'Sprawdź dostęp' }));
		await waitFor(() => expect(goto).toHaveBeenCalledWith('/'));
		expect(h.fbUser.getIdToken).toHaveBeenCalledWith(true);
	});

	it('stays while the email is still unverified', async () => {
		vi.mocked(getMe).mockResolvedValue(user() as never);
		render(PendingPage);
		await fireEvent.click(screen.getByRole('button', { name: 'Sprawdź dostęp' }));
		expect(await screen.findByRole('status')).toHaveTextContent(
			'Adres e-mail nie jest jeszcze potwierdzony.'
		);
		expect(goto).not.toHaveBeenCalled();
	});

	it('explains a suspension without offering verification', () => {
		h.store.user = user({ email_verified: true, suspended: true });
		render(PendingPage);
		expect(screen.getByRole('heading', { name: 'Konto zawieszone' })).toBeVisible();
		expect(screen.queryByRole('button', { name: 'Wyślij link weryfikacyjny' })).toBeNull();
		expect(screen.getByRole('button', { name: 'Wyloguj się' })).toBeVisible();
	});

	it('signs out', async () => {
		render(PendingPage);
		await fireEvent.click(screen.getByRole('button', { name: 'Wyloguj się' }));
		await waitFor(() => expect(goto).toHaveBeenCalledWith('/login'));
		expect(h.logout).toHaveBeenCalled();
	});
});
