import { it, expect, vi } from 'vitest';
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

import { listYachts, createYacht } from '#lib/api/routes.js';
import YachtsPage from './+page.svelte';

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
