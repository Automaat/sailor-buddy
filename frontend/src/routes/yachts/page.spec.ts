import { beforeEach, describe, it, expect, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import type { Yacht } from '#lib/api/types.js';

vi.mock('#lib/api/routes.js', () => ({
	listYachts: vi.fn(),
	createYacht: vi.fn(),
	deleteYacht: vi.fn()
}));
vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { isAdmin: true, user: { id: 1, role: 'admin' } }
}));
vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

import { auth } from '#lib/stores/auth.svelte.js';
import { goto } from '$app/navigation';
import { listYachts, createYacht, deleteYacht } from '#lib/api/routes.js';
import { ApiError } from '#lib/api/client.js';
import YachtsPage from './+page.svelte';

const mockAuth = auth as unknown as {
	isAdmin: boolean;
	user: { id: number; role: string; is_captain?: boolean } | null;
};

beforeEach(() => {
	vi.mocked(listYachts).mockReset();
	vi.mocked(goto).mockClear();
	mockAuth.isAdmin = true;
	mockAuth.user = { id: 1, role: 'admin' };
});

describe('access', () => {
	it('sends a member outside the captain group to the dashboard without loading yachts', async () => {
		mockAuth.isAdmin = false;
		mockAuth.user = { id: 7, role: 'member', is_captain: false };
		render(YachtsPage);

		await waitFor(() => expect(goto).toHaveBeenCalledWith('/'));
		expect(listYachts).not.toHaveBeenCalled();
		expect(screen.queryByRole('heading', { name: 'Jachty' })).toBeNull();
	});

	it('waits for the account before deciding', () => {
		mockAuth.isAdmin = false;
		mockAuth.user = null;
		render(YachtsPage);

		expect(goto).not.toHaveBeenCalled();
		expect(listYachts).not.toHaveBeenCalled();
		expect(screen.queryByRole('heading', { name: 'Jachty' })).toBeNull();
	});

	it('shows the list read-only to the captain group', async () => {
		mockAuth.isAdmin = false;
		mockAuth.user = { id: 8, role: 'member', is_captain: true };
		vi.mocked(listYachts).mockResolvedValue([{ id: 5, name: 'Zawisza' } as Yacht]);
		render(YachtsPage);

		expect(await screen.findByText('Zawisza')).toBeInTheDocument();
		expect(screen.getByRole('heading', { name: 'Jachty' })).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: '+ Dodaj jacht' })).toBeNull();
		expect(screen.queryByRole('button', { name: 'Usuń' })).toBeNull();
		expect(goto).not.toHaveBeenCalled();
	});

	it('shows the list with controls to an admin', async () => {
		vi.mocked(listYachts).mockResolvedValue([{ id: 5, name: 'Zawisza' } as Yacht]);
		render(YachtsPage);

		expect(await screen.findByRole('button', { name: 'Usuń' })).toBeInTheDocument();
		expect(screen.getByRole('button', { name: '+ Dodaj jacht' })).toBeInTheDocument();
		expect(goto).not.toHaveBeenCalled();
	});
});

function deferred<T>() {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((r) => (resolve = r));
	return { promise, resolve };
}

it('shows a yacht once when the list already holds it before its create returns', async () => {
	const yacht = { id: 5, name: 'Zawisza' } as Yacht;
	const list = deferred<Yacht[]>();
	const create = deferred<Yacht>();
	vi.mocked(listYachts).mockReturnValue(list.promise);
	vi.mocked(createYacht).mockReturnValue(create.promise);

	render(YachtsPage);
	await fireEvent.click(screen.getByRole('button', { name: '+ Dodaj jacht' }));
	await fireEvent.input(screen.getByLabelText('Nazwa *'), { target: { value: 'Zawisza' } });
	await fireEvent.click(screen.getByRole('button', { name: 'Dodaj' }));

	list.resolve([yacht]);
	await waitFor(() => expect(screen.getAllByText('Zawisza')).toHaveLength(1));
	create.resolve(yacht);

	await waitFor(() => expect(screen.queryByLabelText('Nazwa *')).toBeNull());
	expect(screen.getAllByText('Zawisza')).toHaveLength(1);
});

it('removes a deleted yacht after confirmation', async () => {
	vi.mocked(listYachts).mockResolvedValue([{ id: 5, name: 'Zawisza' } as Yacht]);
	vi.mocked(deleteYacht).mockReset().mockResolvedValue(undefined);
	const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
	render(YachtsPage);

	await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));
	await waitFor(() => expect(screen.queryByText('Zawisza')).toBeNull());
	expect(deleteYacht).toHaveBeenCalledWith(5);
	expect(screen.queryByRole('alert')).toBeNull();
	confirmSpy.mockRestore();
});

it('shows the error and keeps the yacht when deletion fails', async () => {
	vi.mocked(listYachts).mockResolvedValue([{ id: 5, name: 'Zawisza' } as Yacht]);
	vi.mocked(deleteYacht).mockReset().mockRejectedValue(new ApiError(409, 'yacht is in use'));
	const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
	const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
	render(YachtsPage);

	await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));
	expect(await screen.findByRole('alert')).toHaveTextContent(
		'Nie udało się usunąć jachtu: yacht is in use'
	);
	expect(screen.getByText('Zawisza')).toBeInTheDocument();
	confirmSpy.mockRestore();
	errorSpy.mockRestore();
});

it('clears a yacht delete error when a retry succeeds', async () => {
	vi.mocked(listYachts).mockResolvedValue([{ id: 5, name: 'Zawisza' } as Yacht]);
	vi.mocked(deleteYacht)
		.mockReset()
		.mockRejectedValueOnce(new Error('offline'))
		.mockResolvedValueOnce(undefined);
	const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
	const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
	render(YachtsPage);

	await fireEvent.click(await screen.findByRole('button', { name: 'Usuń' }));
	expect(await screen.findByRole('alert')).toHaveTextContent('offline');
	await fireEvent.click(screen.getByRole('button', { name: 'Usuń' }));

	await waitFor(() => expect(screen.queryByText('Zawisza')).toBeNull());
	expect(screen.queryByRole('alert')).toBeNull();
	confirmSpy.mockRestore();
	errorSpy.mockRestore();
});
