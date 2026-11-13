import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { makePerson } from '#lib/test-utils.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: { params: { id: '4' } } }));
vi.mock('#lib/api/routes.js', () => ({
	getCrew: vi.fn(),
	updateCrew: vi.fn(),
	deleteCrew: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));

import { getCrew, updateCrew } from '#lib/api/routes.js';
import PersonPage from './+page.svelte';

const getMock = vi.mocked(getCrew);
const updateMock = vi.mocked(updateCrew);

beforeEach(() => {
	getMock.mockReset();
	updateMock.mockReset().mockResolvedValue(undefined);
});

describe('person page', () => {
	it('edits other names of a person without an account', async () => {
		getMock.mockResolvedValueOnce(
			makePerson({ id: 4, full_name: 'Ewa Maj', other_names: ['Ewa Kos'], pzz_license_type: 'A' })
		);
		getMock.mockResolvedValueOnce(
			makePerson({ id: 4, full_name: 'Ewa Maj', other_names: ['Ewa Kos', 'Ewa Majówna'] })
		);
		render(PersonPage);
		await screen.findByRole('heading', { name: 'Ewa Maj' });
		expect(screen.getByText('bez konta')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Usuń' })).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Edytuj' }));
		const other = screen.getByLabelText('Inne nazwiska / pisownie');
		expect(other).toHaveValue('Ewa Kos');
		await fireEvent.input(other, { target: { value: 'Ewa Kos, Ewa Majówna' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Zapisz' }));

		await waitFor(() =>
			expect(updateMock).toHaveBeenCalledWith(
				4,
				expect.objectContaining({
					full_name: 'Ewa Maj',
					other_names: ['Ewa Kos', 'Ewa Majówna'],
					pzz_license_type: 'A'
				})
			)
		);
		await screen.findByText('Ewa Kos, Ewa Majówna');
	});

	it('hides delete for a person with an account', async () => {
		getMock.mockResolvedValue(makePerson({ id: 4, full_name: 'Anna Nowak', has_account: true, user_id: 5 }));
		render(PersonPage);
		await screen.findByText('ma konto');
		expect(screen.queryByRole('button', { name: 'Usuń' })).not.toBeInTheDocument();
	});
});
