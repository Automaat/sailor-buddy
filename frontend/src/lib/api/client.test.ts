import { describe, it, expect, vi, beforeEach } from 'vitest';
import { jsonResponse } from '$lib/test-utils';
import type { Page } from './types';

vi.mock('$lib/stores/auth.svelte', () => ({
	auth: { getIdToken: vi.fn(), logout: vi.fn() }
}));

import { auth } from '$lib/stores/auth.svelte';
import { api } from './client';

const getIdToken = auth.getIdToken as unknown as ReturnType<typeof vi.fn>;
const logout = auth.logout as unknown as ReturnType<typeof vi.fn>;
const fetchMock = vi.fn();

beforeEach(() => {
	getIdToken.mockReset().mockResolvedValue('test-token');
	logout.mockReset().mockResolvedValue(undefined);
	fetchMock.mockReset();
	vi.stubGlobal('fetch', fetchMock);
});

function page<T>(items: T[], hasMore: boolean): Page<T> {
	return { items, total: items.length, limit: 100, offset: 0, has_more: hasMore };
}

describe('api.get', () => {
	it('substitutes path params and sends the bearer token', async () => {
		fetchMock.mockResolvedValue(jsonResponse({ id: 1 }));
		await api.get('/trips/{tripID}', { path: { tripID: 1 } });

		expect(fetchMock).toHaveBeenCalledWith('/api/trips/1', {
			method: 'GET',
			headers: {
				'Content-Type': 'application/json',
				Authorization: 'Bearer test-token'
			},
			body: undefined
		});
	});

	it('omits the Authorization header when no token is available', async () => {
		getIdToken.mockResolvedValue(null);
		fetchMock.mockResolvedValue(jsonResponse({}));
		await api.get('/dashboard');

		const headers = fetchMock.mock.calls[0][1].headers as Record<string, string>;
		expect(headers.Authorization).toBeUndefined();
	});

	it('returns the parsed JSON body', async () => {
		fetchMock.mockResolvedValue(jsonResponse({ name: 'Rejs' }));
		await expect(api.get('/trips/{tripID}', { path: { tripID: 1 } })).resolves.toEqual({
			name: 'Rejs'
		});
	});
});

describe('request error handling', () => {
	it('logs out and throws on a 401 response', async () => {
		fetchMock.mockResolvedValue(jsonResponse({}, { status: 401 }));
		await expect(api.get('/dashboard')).rejects.toThrow('Session expired');
		expect(logout).toHaveBeenCalled();
	});

	it('throws the detail field from an error body', async () => {
		fetchMock.mockResolvedValue(jsonResponse({ detail: 'Brak dostępu' }, { status: 403 }));
		await expect(api.get('/dashboard')).rejects.toThrow('Brak dostępu');
	});

	it('falls back to the title field when detail is absent', async () => {
		fetchMock.mockResolvedValue(jsonResponse({ title: 'Not Found' }, { status: 404 }));
		await expect(api.get('/trips/{tripID}', { path: { tripID: 9 } })).rejects.toThrow('Not Found');
	});

	it('falls back to a generic message when the error body is empty', async () => {
		fetchMock.mockResolvedValue(jsonResponse({}, { status: 500 }));
		await expect(api.get('/dashboard')).rejects.toThrow('Request failed: 500');
	});

	it('returns undefined for a 204 No Content response', async () => {
		fetchMock.mockResolvedValue(jsonResponse(null, { status: 204 }));
		await expect(api.del('/trips/{tripID}', { path: { tripID: 1 } })).resolves.toBeUndefined();
	});
});

describe('api write verbs', () => {
	it('serializes the body and uses POST', async () => {
		fetchMock.mockResolvedValue(jsonResponse({ id: 2 }));
		await api.post('/trips', { body: { name: 'Nowy' } });

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/trips',
			expect.objectContaining({ method: 'POST', body: JSON.stringify({ name: 'Nowy' }) })
		);
	});

	it('sends an undefined body when POST has no payload', async () => {
		fetchMock.mockResolvedValue(jsonResponse({}));
		await api.post('/trips/{tripID}/cancel', { path: { tripID: 1 } });

		expect(fetchMock.mock.calls[0][0]).toBe('/api/trips/1/cancel');
		expect(fetchMock.mock.calls[0][1].body).toBeUndefined();
	});

	it('serializes the body and uses PUT', async () => {
		fetchMock.mockResolvedValue(jsonResponse({}, { status: 204 }));
		await api.put('/trips/{tripID}', { path: { tripID: 1 }, body: { name: 'Edytowany' } });

		expect(fetchMock).toHaveBeenCalledWith(
			'/api/trips/1',
			expect.objectContaining({ method: 'PUT', body: JSON.stringify({ name: 'Edytowany' }) })
		);
	});

	it('uses DELETE for del', async () => {
		fetchMock.mockResolvedValue(jsonResponse(null, { status: 204 }));
		await api.del('/trips/{tripID}', { path: { tripID: 1 } });

		expect(fetchMock.mock.calls[0][1].method).toBe('DELETE');
	});
});

describe('api.list pagination', () => {
	it('returns the items of a single page', async () => {
		fetchMock.mockResolvedValue(jsonResponse(page([{ id: 1 }, { id: 2 }], false)));
		await expect(api.list('/trips')).resolves.toEqual([{ id: 1 }, { id: 2 }]);
		expect(fetchMock).toHaveBeenCalledWith('/api/trips?limit=100&offset=0', expect.anything());
	});

	it('walks every page, advancing the offset by the items returned', async () => {
		const first = Array.from({ length: 100 }, (_, i) => ({ id: i }));
		const second = Array.from({ length: 30 }, (_, i) => ({ id: 100 + i }));
		fetchMock
			.mockResolvedValueOnce(jsonResponse(page(first, true)))
			.mockResolvedValueOnce(jsonResponse(page(second, false)));

		const all = await api.list('/voyages');

		expect(all).toHaveLength(130);
		expect(fetchMock).toHaveBeenCalledTimes(2);
		expect(fetchMock.mock.calls[1][0]).toBe('/api/voyages?limit=100&offset=100');
	});

	it('stops when a page returns no items even if has_more is true', async () => {
		fetchMock.mockResolvedValue(jsonResponse(page([], true)));
		await expect(api.list('/trips')).resolves.toEqual([]);
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});
});

describe('api.upload', () => {
	it('posts FormData with the bearer token and no Content-Type header', async () => {
		fetchMock.mockResolvedValue(jsonResponse({ url: '/uploads/x.png' }));
		const fd = new FormData();
		fd.append('file', new File(['x'], 'x.png'));

		await api.upload('/upload/image', fd);

		const [url, init] = fetchMock.mock.calls[0];
		expect(url).toBe('/api/upload/image');
		expect(init.method).toBe('POST');
		expect(init.body).toBe(fd);
		expect((init.headers as Record<string, string>)['Content-Type']).toBeUndefined();
		expect((init.headers as Record<string, string>).Authorization).toBe('Bearer test-token');
	});

	it('logs out and throws on a 401 during upload', async () => {
		fetchMock.mockResolvedValue(jsonResponse({}, { status: 401 }));
		await expect(api.upload('/upload/image', new FormData())).rejects.toThrow('Session expired');
		expect(logout).toHaveBeenCalled();
	});

	it('throws the detail field when an upload fails', async () => {
		fetchMock.mockResolvedValue(jsonResponse({ detail: 'Plik za duży' }, { status: 400 }));
		await expect(api.upload('/upload/image', new FormData())).rejects.toThrow('Plik za duży');
	});
});

// Captured before any spy is installed so the createElement spy can delegate
// to the genuine implementation without recursing into itself.
const realCreateElement = document.createElement.bind(document);

describe('api.download', () => {
	let lastAnchor: HTMLAnchorElement | undefined;

	function fileResponse(disposition?: string, status = 200): Response {
		return {
			ok: status >= 200 && status < 300,
			status,
			headers: new Headers(disposition ? { 'Content-Disposition': disposition } : {}),
			blob: async () => new Blob(['file-bytes']),
			json: async () => ({})
		} as Response;
	}

	beforeEach(() => {
		lastAnchor = undefined;
		URL.createObjectURL = vi.fn(() => 'blob:fake');
		URL.revokeObjectURL = vi.fn();
		vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
		vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
			const el = realCreateElement(tag);
			if (tag === 'a') lastAnchor = el as HTMLAnchorElement;
			return el;
		});
	});

	it('downloads with the filename from the Content-Disposition header', async () => {
		fetchMock.mockResolvedValue(fileResponse('attachment; filename="crew.pdf"'));
		await api.download('/voyages/{voyageID}/opinions/{opinionID}/download', {
			path: { voyageID: 1, opinionID: 2 }
		});

		expect(lastAnchor?.download).toBe('crew.pdf');
		expect(HTMLAnchorElement.prototype.click).toHaveBeenCalled();
		expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:fake');
	});

	it('falls back to the path tail when no Content-Disposition is set', async () => {
		fetchMock.mockResolvedValue(fileResponse());
		await api.download('/voyages/{voyageID}/opinions/{opinionID}/download', {
			path: { voyageID: 1, opinionID: 2 }
		});
		expect(lastAnchor?.download).toBe('download');
	});

	it('logs out and throws on a 401 during download', async () => {
		fetchMock.mockResolvedValue(fileResponse(undefined, 401));
		await expect(
			api.download('/voyages/{voyageID}/opinions/{opinionID}/download', {
				path: { voyageID: 1, opinionID: 2 }
			})
		).rejects.toThrow('Session expired');
		expect(logout).toHaveBeenCalled();
	});

	it('throws the detail field when a download fails', async () => {
		fetchMock.mockResolvedValue(jsonResponse({ detail: 'Nie znaleziono' }, { status: 404 }));
		await expect(
			api.download('/voyages/{voyageID}/opinions/{opinionID}/download', {
				path: { voyageID: 1, opinionID: 2 }
			})
		).rejects.toThrow('Nie znaleziono');
	});
});
