import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

const h = vi.hoisted(() => ({ store: { user: null as unknown } }));

vi.mock('#lib/api/routes.js', () => ({ updateMe: vi.fn() }));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: {
		get user() {
			return h.store.user;
		},
		set user(u: unknown) {
			h.store.user = u;
		},
		get firebaseUser() {
			return { displayName: 'Jan Maria Kowalski' };
		}
	}
}));

import { updateMe } from '#lib/api/routes.js';
import OnboardingPage from './+page.svelte';

const updateMeMock = updateMe as unknown as ReturnType<typeof vi.fn>;

beforeEach(() => {
	h.store.user = { id: 1, name: 'Jan Maria Kowalski', first_name: '', last_name: '', onboarded: false };
	updateMeMock.mockReset().mockImplementation(async (body) => ({ ...body, onboarded: true }));
});

describe('onboarding page', () => {
	it('prefills the name from the account and submits the profile', async () => {
		render(OnboardingPage);
		const first = (await screen.findByLabelText('Imię *')) as HTMLInputElement;
		expect(first.value).toBe('Jan');
		expect((screen.getByLabelText('Nazwisko *') as HTMLInputElement).value).toBe('Maria Kowalski');

		await fireEvent.input(screen.getByLabelText('Telefon'), { target: { value: '+48 600 100 200' } });
		await fireEvent.change(screen.getByLabelText('Patent żeglarski'), { target: { value: 'zeglarz_jachtowy' } });
		await fireEvent.input(screen.getByLabelText('Numer patentu'), { target: { value: 'ZJ-1' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz i przejdź dalej' }));

		await waitFor(() =>
			expect(updateMeMock).toHaveBeenCalledWith({
				first_name: 'Jan',
				last_name: 'Maria Kowalski',
				phone: '+48 600 100 200',
				patent_type: 'zeglarz_jachtowy',
				patent_number: 'ZJ-1'
			})
		);
		await waitFor(() => expect((h.store.user as { onboarded: boolean }).onboarded).toBe(true));
	});

	it('requires a patent number only when a patent is chosen', async () => {
		render(OnboardingPage);
		const number = (await screen.findByLabelText('Numer patentu')) as HTMLInputElement;
		expect(number.disabled).toBe(true);
		expect(number.required).toBe(false);
		await fireEvent.change(screen.getByLabelText('Patent żeglarski'), { target: { value: 'kapitan_jachtowy' } });
		expect(number.disabled).toBe(false);
		expect(number.required).toBe(true);
	});

	it('shows the API error and keeps the user on the form', async () => {
		updateMeMock.mockRejectedValueOnce(new Error('first and last name are required'));
		render(OnboardingPage);
		await screen.findByLabelText('Imię *');
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz i przejdź dalej' }));

		expect(await screen.findByText('first and last name are required')).toBeTruthy();
		expect((h.store.user as { onboarded: boolean }).onboarded).toBe(false);
	});
});
