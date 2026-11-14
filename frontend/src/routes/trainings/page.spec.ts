import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import type { Training } from '#lib/api/types.js';

vi.mock('#lib/api/routes.js', () => ({
	listTrainings: vi.fn(),
	createTraining: vi.fn(),
	deleteTraining: vi.fn()
}));

import { createTraining, deleteTraining, listTrainings } from '#lib/api/routes.js';
import TrainingsPage from './+page.svelte';
import { load } from './+page.js';
import type { PageData } from './$types';

const mockList = vi.mocked(listTrainings);
const mockCreate = vi.mocked(createTraining);
const mockDelete = vi.mocked(deleteTraining);

function training(overrides: Partial<Training> = {}): Training {
	return { id: 1, name: 'Sternik motorowodny', cost: 0, ...overrides } as Training;
}

function runLoad() {
	return load({} as unknown as Parameters<typeof load>[0]) as Promise<PageData>;
}

async function renderPage() {
	return render(TrainingsPage, { props: { data: await runLoad() } });
}

beforeEach(() => {
	mockList.mockReset().mockResolvedValue([training()]);
	mockCreate.mockReset();
	mockDelete.mockReset().mockResolvedValue(undefined);
});

describe('trainings load', () => {
	it('loads the member training log', async () => {
		await expect(runLoad()).resolves.toEqual({ trainings: [training()] });
		expect(mockList).toHaveBeenCalledOnce();
	});

	it('treats a null reply as an empty log', async () => {
		mockList.mockResolvedValue(null as unknown as Training[]);
		await expect(runLoad()).resolves.toEqual({ trainings: [] });
	});

	it('passes a failure on to +error.svelte instead of showing an empty log', async () => {
		const boom = new Error('boom');
		mockList.mockRejectedValue(boom);
		await expect(runLoad()).rejects.toBe(boom);
	});
});

describe('trainings page', () => {
	it('lists the loaded trainings', async () => {
		await renderPage();
		expect(screen.getByText('Sternik motorowodny')).toBeInTheDocument();
	});

	it('shows the empty state for an empty log', async () => {
		mockList.mockResolvedValue([]);
		await renderPage();
		expect(screen.getByText('Brak szkoleń')).toBeInTheDocument();
	});

	it('adds a new training to the top of the list', async () => {
		mockCreate.mockResolvedValue(training({ id: 2, name: 'Jachtowy sternik morski' }));
		await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: '+ Dodaj szkolenie' }));
		await fireEvent.input(screen.getByLabelText('Nazwa *'), { target: { value: 'Jachtowy sternik morski' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj' }));

		expect(await screen.findByText('Jachtowy sternik morski')).toBeInTheDocument();
		expect(screen.getByText('Sternik motorowodny')).toBeInTheDocument();
	});

	it('removes a deleted training after confirmation', async () => {
		vi.spyOn(window, 'confirm').mockReturnValue(true);
		await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: 'Usuń' }));

		await vi.waitFor(() => expect(screen.queryByText('Sternik motorowodny')).not.toBeInTheDocument());
		expect(mockDelete).toHaveBeenCalledWith(1);
	});

	it('replaces a locally changed list with the reloaded log', async () => {
		mockCreate.mockResolvedValue(training({ id: 2, name: 'Jachtowy sternik morski' }));
		const view = await renderPage();
		await fireEvent.click(screen.getByRole('button', { name: '+ Dodaj szkolenie' }));
		await fireEvent.input(screen.getByLabelText('Nazwa *'), { target: { value: 'Jachtowy sternik morski' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj' }));
		expect(await screen.findByText('Jachtowy sternik morski')).toBeInTheDocument();

		mockList.mockResolvedValue([training({ id: 3, name: 'Radiooperator SRC' })]);
		await view.rerender({ data: await runLoad() });

		expect(screen.getByText('Radiooperator SRC')).toBeInTheDocument();
		expect(screen.queryByText('Jachtowy sternik morski')).not.toBeInTheDocument();
		expect(screen.queryByText('Sternik motorowodny')).not.toBeInTheDocument();
	});
});
