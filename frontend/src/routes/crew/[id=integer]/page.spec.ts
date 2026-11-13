import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { makePerson } from '#lib/test-utils.js';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));
vi.mock('$app/state', () => ({ page: { params: { id: '4' } } }));
vi.mock('#lib/api/routes.js', () => ({
	getCrew: vi.fn(),
	updateCrew: vi.fn(),
	deleteCrew: vi.fn(),
	listCrew: vi.fn(),
	previewCrewMerge: vi.fn(),
	mergeCrew: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));

import { getCrew, updateCrew, listCrew, previewCrewMerge, mergeCrew } from '#lib/api/routes.js';
import { goto } from '$app/navigation';
import type { CrewMember, PersonMerge } from '#lib/api/types.js';
import PersonPage from './+page.svelte';

const getMock = vi.mocked(getCrew);
const updateMock = vi.mocked(updateCrew);
const listMock = vi.mocked(listCrew);
const previewMock = vi.mocked(previewCrewMerge);
const mergeMock = vi.mocked(mergeCrew);

beforeEach(() => {
	getMock.mockReset();
	updateMock.mockReset().mockResolvedValue(undefined);
	listMock.mockReset();
	previewMock.mockReset();
	mergeMock.mockReset();
	vi.mocked(goto).mockReset();
});

function mergeOf(person: CrewMember, duplicate: CrewMember, counts: Partial<PersonMerge> = {}): PersonMerge {
	return {
		person,
		duplicate,
		trip_assignments: 0,
		voyage_assignments: 0,
		shared_assignments: 0,
		opinions: 0,
		shared_opinions: 0,
		captained_trips: 0,
		captained_voyages: 0,
		...counts
	};
}

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

	it('merges a historical record into the account person after confirming what moves', async () => {
		const historical = makePerson({ id: 4, full_name: 'Ewa Maj', other_names: ['Ewa Kos'] });
		const account = makePerson({ id: 9, full_name: 'Ewa Majówna', has_account: true, user_id: 3 });
		getMock.mockResolvedValue(historical);
		listMock.mockResolvedValue([historical, account, makePerson({ id: 7, full_name: 'Jan Kowalski' })]);
		const kept = { ...account, other_names: ['Ewa Maj', 'Ewa Kos'] };
		previewMock.mockResolvedValue(
			mergeOf(kept, historical, {
				trip_assignments: 2,
				voyage_assignments: 5,
				shared_assignments: 1,
				opinions: 1,
				captained_voyages: 2
			})
		);
		mergeMock.mockResolvedValue(mergeOf(kept, historical));

		render(PersonPage);
		await screen.findByRole('heading', { name: 'Ewa Maj' });
		await fireEvent.click(screen.getByRole('button', { name: 'Połącz z…' }));
		await fireEvent.input(await screen.findByLabelText('Szukaj osoby'), { target: { value: 'majowna' } });
		expect(screen.queryByRole('button', { name: /Jan Kowalski/ })).not.toBeInTheDocument();
		await fireEvent.click(screen.getByRole('button', { name: /Ewa Majówna/ }));

		await waitFor(() => expect(previewMock).toHaveBeenCalledWith(9, 4));
		expect(screen.getByRole('radio', { name: 'Ewa Majówna (ma konto)' })).toBeChecked();
		const summary = await screen.findByTestId('merge-summary');
		expect(summary).toHaveTextContent('2 przypisania do etapów');
		expect(summary).toHaveTextContent('5 przypisań do rejsów');
		expect(summary).toHaveTextContent('1 opinia');
		expect(summary).toHaveTextContent('kapitan: 0 etapów, 2 rejsy');
		expect(summary).toHaveTextContent('Wspólne wpisy obu osób');
		expect(summary).toHaveTextContent('Inne nazwiska po połączeniu: Ewa Maj, Ewa Kos');
		expect(mergeMock).not.toHaveBeenCalled();

		await fireEvent.click(screen.getByRole('button', { name: 'Połącz' }));
		await waitFor(() => expect(mergeMock).toHaveBeenCalledWith(9, 4));
		expect(goto).toHaveBeenCalledWith('/crew/9');
	});

	it('keeps the current person when switched and shows the result in place', async () => {
		const current = makePerson({ id: 4, full_name: 'Ewa Maj' });
		const other = makePerson({ id: 7, full_name: 'Ewa Maj (stara)' });
		getMock.mockResolvedValue(current);
		listMock.mockResolvedValue([current, other]);
		previewMock.mockImplementation(async (keptID) =>
			keptID === 4 ? mergeOf(current, other) : mergeOf(other, current)
		);
		mergeMock.mockResolvedValue(mergeOf({ ...current, other_names: ['Ewa Maj (stara)'] }, other));

		render(PersonPage);
		await screen.findByRole('heading', { name: 'Ewa Maj' });
		await fireEvent.click(screen.getByRole('button', { name: 'Połącz z…' }));
		await fireEvent.click(await screen.findByRole('button', { name: /Ewa Maj \(stara\)/ }));
		await waitFor(() => expect(previewMock).toHaveBeenCalledWith(4, 7));
		await fireEvent.click(screen.getByRole('radio', { name: 'Ewa Maj (stara)' }));
		await waitFor(() => expect(previewMock).toHaveBeenLastCalledWith(7, 4));
		await fireEvent.click(screen.getByRole('radio', { name: 'Ewa Maj' }));
		await waitFor(() => expect(previewMock).toHaveBeenLastCalledWith(4, 7));
		await screen.findByTestId('merge-summary');

		await fireEvent.click(screen.getByRole('button', { name: 'Połącz' }));
		await waitFor(() => expect(mergeMock).toHaveBeenCalledWith(4, 7));
		expect(goto).not.toHaveBeenCalled();
		expect(await screen.findByRole('status')).toHaveTextContent('Połączono: Ewa Maj (stara) → Ewa Maj.');
		expect(screen.getByText('Ewa Maj (stara)')).toBeInTheDocument();
	});

	it('refuses to merge two people with accounts', async () => {
		const current = makePerson({ id: 4, full_name: 'Anna Nowak', has_account: true, user_id: 5 });
		getMock.mockResolvedValue(current);
		listMock.mockResolvedValue([
			current,
			makePerson({ id: 8, full_name: 'Anna Nowakowska', has_account: true, user_id: 6 })
		]);

		render(PersonPage);
		await screen.findByRole('heading', { name: 'Anna Nowak' });
		await fireEvent.click(screen.getByRole('button', { name: 'Połącz z…' }));
		await fireEvent.click(await screen.findByRole('button', { name: /Anna Nowakowska/ }));
		expect(screen.getByText('Obie osoby mają konta — nie można ich połączyć.')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: 'Połącz' })).toBeDisabled();
		expect(previewMock).not.toHaveBeenCalled();
	});

	it('names the kept record by its own name when it takes over the other account', async () => {
		const current = makePerson({ id: 4, full_name: 'Zofia Duplikat' });
		const account = makePerson({ id: 9, full_name: 'Mira Member', has_account: true, user_id: 3 });
		getMock.mockResolvedValue(current);
		listMock.mockResolvedValue([current, account]);
		const renamed = { ...current, full_name: 'Mira Member', other_names: ['Zofia Duplikat'], has_account: true, user_id: 3 };
		previewMock.mockImplementation(async (keptID) =>
			keptID === 4 ? mergeOf(renamed, account) : mergeOf(account, current)
		);
		mergeMock.mockResolvedValue(mergeOf(renamed, account));

		render(PersonPage);
		await screen.findByRole('heading', { name: 'Zofia Duplikat' });
		await fireEvent.click(screen.getByRole('button', { name: 'Połącz z…' }));
		await fireEvent.click(await screen.findByRole('button', { name: /Mira Member/ }));
		await waitFor(() => expect(previewMock).toHaveBeenCalledWith(9, 4));
		await fireEvent.click(screen.getByRole('radio', { name: 'Zofia Duplikat' }));
		await waitFor(() => expect(previewMock).toHaveBeenLastCalledWith(4, 9));
		const summary = await screen.findByTestId('merge-summary');
		expect(summary).toHaveTextContent('Zostanie Zofia Duplikat, a Mira Member zostanie usunięta.');
		expect(summary).toHaveTextContent('Imię i nazwisko po połączeniu (z konta): Mira Member');
		expect(summary).toHaveTextContent('Konto osoby Mira Member zostanie przypisane do Zofia Duplikat.');

		await fireEvent.click(screen.getByRole('button', { name: 'Połącz' }));
		await waitFor(() => expect(mergeMock).toHaveBeenCalledWith(4, 9));
		expect(await screen.findByRole('status')).toHaveTextContent(
			'Połączono: Mira Member → Zofia Duplikat. Imię i nazwisko z konta: Mira Member.'
		);
	});

	it('keeps the selection fixed while a preview is loading', async () => {
		const current = makePerson({ id: 4, full_name: 'Ewa Maj' });
		const first = makePerson({ id: 7, full_name: 'Ewa Pierwsza' });
		getMock.mockResolvedValue(current);
		listMock.mockResolvedValue([current, first]);
		previewMock.mockImplementationOnce(() => new Promise(() => {}));

		render(PersonPage);
		await screen.findByRole('heading', { name: 'Ewa Maj' });
		await fireEvent.click(screen.getByRole('button', { name: 'Połącz z…' }));
		await fireEvent.click(await screen.findByRole('button', { name: /Ewa Pierwsza/ }));
		expect(screen.getByRole('button', { name: 'Wybierz inną osobę' })).toBeDisabled();
		expect(screen.getByRole('button', { name: 'Anuluj' })).toBeDisabled();
	});
});
