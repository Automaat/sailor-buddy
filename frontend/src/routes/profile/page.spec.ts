import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

const h = vi.hoisted(() => ({
	fbUser: {
		displayName: 'Jan Kowalski',
		photoURL: '',
		email: 'jan@example.com',
		providerData: [{ providerId: 'password' }] as { providerId: string }[],
		getIdToken: vi.fn()
	},
	store: { user: null as unknown }
}));

vi.mock('firebase/auth', () => ({
	updateProfile: vi.fn().mockResolvedValue(undefined),
	updatePassword: vi.fn().mockResolvedValue(undefined),
	reauthenticateWithCredential: vi.fn().mockResolvedValue(undefined),
	EmailAuthProvider: { credential: vi.fn(() => ({})) }
}));

vi.mock('#lib/api/routes.js', () => ({
	updateMe: vi.fn(),
	getMyPersonLinkRequest: vi.fn(),
	listPersonClaimCandidates: vi.fn(),
	createPersonLinkRequest: vi.fn(),
	cancelPersonLinkRequest: vi.fn()
}));

vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: {
		get user() {
			return h.store.user;
		},
		set user(u: unknown) {
			h.store.user = u;
		},
		get firebaseUser() {
			return h.fbUser;
		}
	}
}));

import { updateProfile } from 'firebase/auth';
import {
	updateMe,
	getMyPersonLinkRequest,
	listPersonClaimCandidates,
	createPersonLinkRequest,
	cancelPersonLinkRequest
} from '#lib/api/routes.js';
import ProfilePage from './+page.svelte';

const updateMeMock = updateMe as unknown as ReturnType<typeof vi.fn>;
const myRequestMock = getMyPersonLinkRequest as unknown as ReturnType<typeof vi.fn>;
const candidatesMock = listPersonClaimCandidates as unknown as ReturnType<typeof vi.fn>;
const createRequestMock = createPersonLinkRequest as unknown as ReturnType<typeof vi.fn>;
const cancelRequestMock = cancelPersonLinkRequest as unknown as ReturnType<typeof vi.fn>;

const pastPerson = {
	id: 40,
	full_name: 'Jan Kowalski',
	other_names: ['J. Kowalski'],
	email_match: true
};

function linkRequest(status: string) {
	return {
		id: 7,
		user_id: 1,
		person_id: 40,
		person_name: 'Jan Kowalski',
		status,
		created_at: '2026-01-01T00:00:00Z'
	};
}
const updateProfileMock = updateProfile as unknown as ReturnType<typeof vi.fn>;

function seededUser(patentType: string | undefined, patentNumber: string | undefined) {
	return {
		id: 1,
		email: 'jan@example.com',
		name: 'Jan Kowalski',
		first_name: 'Jan',
		last_name: 'Kowalski',
		phone: '+48 600 100 200',
		onboarded: true,
		avatar_url: '',
		patent_type: patentType,
		patent_number: patentNumber
	};
}

beforeEach(() => {
	myRequestMock.mockReset().mockResolvedValue(null);
	candidatesMock.mockReset().mockResolvedValue([pastPerson]);
	createRequestMock.mockReset().mockResolvedValue(linkRequest('pending'));
	cancelRequestMock.mockReset().mockResolvedValue(undefined);
	h.fbUser.displayName = 'Jan Kowalski';
	h.fbUser.photoURL = '';
	h.fbUser.providerData = [{ providerId: 'password' }];
	h.fbUser.getIdToken.mockReset().mockResolvedValue('token');
	h.store.user = seededUser('kapitan_jachtowy', 'PL-9');
	updateMeMock.mockReset().mockImplementation(async () => h.store.user);
	updateProfileMock.mockClear();
});

describe('profile page', () => {
	it('seeds the patent fields from the stored user', async () => {
		render(ProfilePage);
		const type = (await screen.findByLabelText('Patent żeglarski')) as HTMLSelectElement;
		const number = screen.getByLabelText('Numer patentu') as HTMLInputElement;
		await waitFor(() => expect(type.value).toBe('kapitan_jachtowy'));
		expect(number.value).toBe('PL-9');
	});

	it('saves the selected patent type and number', async () => {
		render(ProfilePage);
		await screen.findByLabelText('Patent żeglarski');
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz' }));

		await waitFor(() =>
			expect(updateMeMock).toHaveBeenCalledWith({
				first_name: 'Jan',
				last_name: 'Kowalski',
				phone: '+48 600 100 200',
				patent_type: 'kapitan_jachtowy',
				patent_number: 'PL-9'
			})
		);
		expect(updateProfileMock).toHaveBeenCalledWith(h.fbUser, {
			displayName: 'Jan Kowalski',
			photoURL: null
		});
	});

	it('drops the patent number when the type is cleared', async () => {
		render(ProfilePage);
		const type = (await screen.findByLabelText('Patent żeglarski')) as HTMLSelectElement;
		await waitFor(() => expect(type.value).toBe('kapitan_jachtowy'));

		await fireEvent.change(type, { target: { value: '' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz' }));

		await waitFor(() =>
			expect(updateMeMock).toHaveBeenCalledWith(
				expect.objectContaining({ patent_type: undefined, patent_number: undefined })
			)
		);
	});

	it('saves edited name and phone', async () => {
		render(ProfilePage);
		const first = (await screen.findByLabelText('Imię *')) as HTMLInputElement;
		await waitFor(() => expect(first.value).toBe('Jan'));
		await fireEvent.input(first, { target: { value: 'Janusz' } });
		await fireEvent.input(screen.getByLabelText('Telefon'), { target: { value: '' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz' }));

		await waitFor(() =>
			expect(updateMeMock).toHaveBeenCalledWith(
				expect.objectContaining({ first_name: 'Janusz', last_name: 'Kowalski', phone: undefined })
			)
		);
	});

	it('shows the API error and skips the Firebase update when saving fails', async () => {
		updateMeMock.mockRejectedValueOnce(new Error('patent number is required'));
		render(ProfilePage);
		await screen.findByLabelText('Patent żeglarski');
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz' }));

		expect(await screen.findByText(/patent number is required/)).toBeTruthy();
		expect(updateProfileMock).not.toHaveBeenCalled();
	});

	it('hides the password section for non-password accounts', async () => {
		h.fbUser.providerData = [{ providerId: 'google.com' }];
		render(ProfilePage);
		await screen.findByLabelText('Patent żeglarski');
		expect(screen.queryByText('Zmień hasło')).toBeNull();
	});
});

describe('profile person claim', () => {
	it('shows a pending request and cancels it', async () => {
		myRequestMock.mockResolvedValue(linkRequest('pending'));
		render(ProfilePage);
		expect(await screen.findByText(/czeka na zatwierdzenie/)).toBeTruthy();
		expect(screen.queryByLabelText('Szukaj osoby')).toBeNull();

		await fireEvent.click(screen.getByRole('button', { name: 'Anuluj prośbę' }));

		await waitFor(() => expect(cancelRequestMock).toHaveBeenCalled());
		expect(await screen.findByLabelText('Szukaj osoby')).toBeTruthy();
		expect(await screen.findByRole('button', { name: 'To ja: Jan Kowalski' })).toBeTruthy();
	});

	it('sends a request and shows the conflict message on 409', async () => {
		const { ApiError } = await import('#lib/api/client.js');
		createRequestMock.mockRejectedValueOnce(new ApiError(409, 'conflict'));
		render(ProfilePage);
		await fireEvent.click(await screen.findByRole('button', { name: 'To ja: Jan Kowalski' }));
		expect(await screen.findByText(/ma już konto albo masz już oczekującą prośbę/)).toBeTruthy();
	});

	it('tells the user about a rejected request and lets them try again', async () => {
		myRequestMock.mockResolvedValue(linkRequest('rejected'));
		render(ProfilePage);
		expect(await screen.findByText(/odrzucił prośbę/)).toBeTruthy();
		expect(await screen.findByRole('button', { name: 'To ja: Jan Kowalski' })).toBeTruthy();
	});

	it('shows the linked person once approved', async () => {
		myRequestMock.mockResolvedValue(linkRequest('approved'));
		render(ProfilePage);
		expect(await screen.findByText(/Konto połączone z osobą/)).toBeTruthy();
		expect(screen.queryByRole('button', { name: /To ja/ })).toBeNull();
	});
});
