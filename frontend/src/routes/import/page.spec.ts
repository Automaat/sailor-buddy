import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import { jsonResponse } from '$lib/test-utils';

vi.mock('$lib/stores/auth.svelte', () => ({
	auth: { getIdToken: vi.fn() }
}));

import { auth } from '$lib/stores/auth.svelte';
import ImportPage from './+page.svelte';

const getIdToken = auth.getIdToken as unknown as ReturnType<typeof vi.fn>;
const fetchMock = vi.fn();

beforeEach(() => {
	getIdToken.mockReset().mockResolvedValue('test-token');
	fetchMock.mockReset();
	vi.stubGlobal('fetch', fetchMock);
});

function pickFile(container: HTMLElement, name = 'sailing.xlsx') {
	const input = container.querySelector('input[type="file"]') as HTMLInputElement;
	const file = new File(['data'], name, {
		type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
	});
	return fireEvent.change(input, { target: { files: [file] } });
}

describe('import page', () => {
	it('starts idle with an upload prompt', () => {
		render(ImportPage);
		expect(screen.getByRole('button', { name: 'Wybierz plik i prześlij' })).toBeInTheDocument();
	});

	it('shows a parsed preview after a successful upload', async () => {
		fetchMock.mockResolvedValueOnce(
			jsonResponse({ voyages: [{}, {}], trainings: [{}] })
		);
		const { container } = render(ImportPage);
		await pickFile(container);

		expect(await screen.findByText('Podgląd importu')).toBeInTheDocument();
		expect(screen.getByText(/Znaleziono 2 rejsów, 1 szkoleń/)).toBeInTheDocument();
		expect(fetchMock).toHaveBeenCalledWith('/api/import/xlsx', expect.objectContaining({ method: 'POST' }));
	});

	it('surfaces a server validation error from the upload response', async () => {
		fetchMock.mockResolvedValueOnce(
			jsonResponse({ detail: 'Nieprawidłowy format pliku' }, { status: 400 })
		);
		const { container } = render(ImportPage);
		await pickFile(container);

		expect(await screen.findByText('Nieprawidłowy format pliku')).toBeInTheDocument();
	});

	it('confirms the import and shows the done state', async () => {
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ voyages: [{}], trainings: [] }))
			.mockResolvedValueOnce(jsonResponse({}));
		const { container } = render(ImportPage);
		await pickFile(container);

		await fireEvent.click(await screen.findByRole('button', { name: 'Potwierdź import' }));

		expect(await screen.findByText('Import zakończony!')).toBeInTheDocument();
		expect(fetchMock).toHaveBeenLastCalledWith('/api/import/confirm', expect.objectContaining({ method: 'POST' }));
	});

	it('shows an error when the confirm step fails', async () => {
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ voyages: [{}], trainings: [] }))
			.mockResolvedValueOnce(jsonResponse({ detail: 'Import nie powiódł się' }, { status: 500 }));
		const { container } = render(ImportPage);
		await pickFile(container);

		await fireEvent.click(await screen.findByRole('button', { name: 'Potwierdź import' }));

		expect(await screen.findByText('Import nie powiódł się')).toBeInTheDocument();
	});

	it('falls back to a generic status message when the error body is empty', async () => {
		fetchMock.mockResolvedValueOnce(jsonResponse({}, { status: 400 }));
		const { container } = render(ImportPage);
		await pickFile(container);

		expect(await screen.findByText('Request failed: 400')).toBeInTheDocument();
	});

	it('falls back to a generic message when the upload rejects with a non-Error', async () => {
		fetchMock.mockRejectedValueOnce('boom');
		const { container } = render(ImportPage);
		await pickFile(container);

		expect(await screen.findByText('Przesyłanie nie powiodło się')).toBeInTheDocument();
	});

	it('falls back to a generic message when the confirm rejects with a non-Error', async () => {
		fetchMock
			.mockResolvedValueOnce(jsonResponse({ voyages: [{}], trainings: [] }))
			.mockRejectedValueOnce('boom');
		const { container } = render(ImportPage);
		await pickFile(container);

		await fireEvent.click(await screen.findByRole('button', { name: 'Potwierdź import' }));

		expect(await screen.findByText('Import nie powiódł się')).toBeInTheDocument();
	});

	it('counts zero rows when the preview has no voyages or trainings', async () => {
		fetchMock.mockResolvedValueOnce(jsonResponse({}));
		const { container } = render(ImportPage);
		await pickFile(container);

		expect(await screen.findByText(/Znaleziono 0 rejsów, 0 szkoleń/)).toBeInTheDocument();
	});

	it('returns to idle when the preview is cancelled', async () => {
		fetchMock.mockResolvedValueOnce(jsonResponse({ voyages: [{}], trainings: [] }));
		const { container } = render(ImportPage);
		await pickFile(container);

		await fireEvent.click(await screen.findByRole('button', { name: 'Anuluj' }));

		expect(screen.getByRole('button', { name: 'Wybierz plik i prześlij' })).toBeInTheDocument();
	});
});
