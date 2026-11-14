import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/svelte';
import { jsonResponse, makePerson, pagedListServer } from '#lib/test-utils.js';

vi.mock('#lib/stores/auth.svelte.js', () => ({
	auth: { getIdToken: vi.fn(() => Promise.resolve('test-token')), logout: vi.fn() }
}));

import CaptainSelect from './CaptainSelect.svelte';

const fetchMock = vi.fn();

function person(id: number) {
	return makePerson({ id, full_name: `Osoba ${String(id).padStart(3, '0')}` });
}

function crewPage(ids: number[], offset: number, hasMore: boolean) {
	return jsonResponse({
		items: ids.map(person),
		total: 155,
		limit: 100,
		offset,
		has_more: hasMore
	});
}

beforeEach(() => {
	fetchMock.mockReset();
	vi.stubGlobal('fetch', fetchMock);
});

describe('CaptainSelect with a paged crew list', () => {
	it('renders 150+ people once each when pages overlap', async () => {
		const first = Array.from({ length: 100 }, (_, i) => i + 1);
		const second = Array.from({ length: 60 }, (_, i) => i + 96);
		fetchMock.mockImplementation((url: string) => {
			if (url.startsWith('/api/members')) return Promise.resolve(jsonResponse([]));
			if (url.includes('offset=0')) return Promise.resolve(crewPage(first, 0, true));
			return Promise.resolve(crewPage(second, 100, false));
		});
		const errors = vi.spyOn(console, 'error').mockImplementation(() => {});

		render(CaptainSelect, { value: 0 });

		await screen.findByRole('option', { name: 'Osoba 155' });
		const options = within(screen.getByRole('combobox')).getAllByRole('option');
		expect(options).toHaveLength(156);
		expect(new Set(options.map((o) => o.getAttribute('value'))).size).toBe(156);
		expect(screen.queryByRole('alert')).toBeNull();
		expect(errors).not.toHaveBeenCalled();
		errors.mockRestore();
	});

	it('lists every existing person when one is deleted between page requests', async () => {
		const server = pagedListServer(Array.from({ length: 160 }, (_, i) => person(i + 1)));
		let crewRequests = 0;
		fetchMock.mockImplementation((url: string) => {
			if (url.startsWith('/api/members')) return Promise.resolve(jsonResponse([]));
			const res = server.respond(url);
			if (++crewRequests === 1) server.remove(50);
			return Promise.resolve(res);
		});

		render(CaptainSelect, { value: 0 });

		await screen.findByRole('option', { name: 'Osoba 160' });
		const values = within(screen.getByRole('combobox'))
			.getAllByRole('option')
			.map((o) => o.getAttribute('value'));
		for (const id of server.ids()) expect(values).toContain(String(id));
		expect(new Set(values).size).toBe(values.length);
	});
});
