import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';

const h = vi.hoisted(() => ({ store: { user: null as unknown } }));

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
			return { displayName: 'Jan Maria Kowalski' };
		}
	}
}));

import {
	updateMe,
	getMyPersonLinkRequest,
	listPersonClaimCandidates,
	createPersonLinkRequest,
	cancelPersonLinkRequest
} from '#lib/api/routes.js';
import OnboardingPage from './+page.svelte';

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

beforeEach(() => {
	myRequestMock.mockReset().mockResolvedValue(null);
	candidatesMock.mockReset().mockResolvedValue([pastPerson]);
	createRequestMock.mockReset().mockResolvedValue(linkRequest('pending'));
	cancelRequestMock.mockReset().mockResolvedValue(undefined);
	h.store.user = {
		id: 1,
		name: 'Jan Maria Kowalski',
		first_name: '',
		last_name: '',
		onboarded: false
	};
	updateMeMock.mockReset().mockImplementation(async (body) => ({ ...body, onboarded: true }));
});

describe('onboarding page', () => {
	it('prefills the name from the account and submits the profile', async () => {
		render(OnboardingPage);
		const first = (await screen.findByLabelText('Imię *')) as HTMLInputElement;
		expect(first.value).toBe('Jan');
		expect((screen.getByLabelText('Nazwisko *') as HTMLInputElement).value).toBe('Maria Kowalski');

		await fireEvent.input(screen.getByLabelText('Telefon'), {
			target: { value: '+48 600 100 200' }
		});
		await fireEvent.change(screen.getByLabelText('Patent żeglarski'), {
			target: { value: 'zeglarz_jachtowy' }
		});
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
		await fireEvent.change(screen.getByLabelText('Patent żeglarski'), {
			target: { value: 'kapitan_jachtowy' }
		});
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

describe('onboarding person claim', () => {
	it('suggests people by e-mail and sends a "To ja" request without saving the profile', async () => {
		render(OnboardingPage);
		expect(await screen.findByText(/pasuje do Twojego e-maila/)).toBeTruthy();
		expect(candidatesMock).toHaveBeenCalledWith('');

		await fireEvent.click(screen.getByRole('button', { name: 'To ja: Jan Kowalski' }));

		await waitFor(() => expect(createRequestMock).toHaveBeenCalledWith(40));
		expect(await screen.findByText(/czeka na zatwierdzenie/)).toBeTruthy();
		expect(updateMeMock).not.toHaveBeenCalled();
		expect((h.store.user as { onboarded: boolean }).onboarded).toBe(false);
	});

	it('searches people by name', async () => {
		candidatesMock
			.mockResolvedValueOnce([])
			.mockResolvedValueOnce([{ ...pastPerson, email_match: false }]);
		render(OnboardingPage);
		const search = await screen.findByLabelText('Szukaj osoby');
		await fireEvent.input(search, { target: { value: 'Kowal' } });

		await waitFor(() => expect(candidatesMock).toHaveBeenLastCalledWith('Kowal'));
		expect(await screen.findByRole('button', { name: 'To ja: Jan Kowalski' })).toBeTruthy();
		expect(screen.queryByText(/pasuje do Twojego e-maila/)).toBeNull();
	});

	it('still lets the user finish onboarding while a request is pending', async () => {
		myRequestMock.mockResolvedValue(linkRequest('pending'));
		render(OnboardingPage);
		expect(await screen.findByText(/czeka na zatwierdzenie/)).toBeTruthy();
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz i przejdź dalej' }));
		await waitFor(() => expect((h.store.user as { onboarded: boolean }).onboarded).toBe(true));
	});
});
