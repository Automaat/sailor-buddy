import { auth } from '$lib/stores/auth.svelte';
import type { Page } from './types';

const BASE = '/api';

async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
	const token = await auth.getIdToken();
	const headers: Record<string, string> = {
		'Content-Type': 'application/json',
		...((opts.headers as Record<string, string>) || {})
	};
	if (token) {
		headers['Authorization'] = `Bearer ${token}`;
	}

	const res = await fetch(`${BASE}${path}`, { ...opts, headers });

	if (res.status === 401) {
		await auth.logout();
		throw new Error('Session expired');
	}

	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new Error(body.detail || body.title || `Request failed: ${res.status}`);
	}

	if (res.status === 204) return undefined as T;
	return res.json();
}

// listAll walks every page of a paginated collection endpoint and returns the
// flattened items, so callers without pagination UI keep getting a full array.
async function listAll<T>(path: string): Promise<T[]> {
	const out: T[] = [];
	const limit = 100;
	let offset = 0;
	for (;;) {
		const sep = path.includes('?') ? '&' : '?';
		const page = await request<Page<T>>(`${path}${sep}limit=${limit}&offset=${offset}`);
		out.push(...page.items);
		if (!page.has_more || page.items.length === 0) break;
		offset += page.items.length;
	}
	return out;
}

async function upload<T>(path: string, formData: FormData): Promise<T> {
	const token = await auth.getIdToken();
	const headers: Record<string, string> = {};
	if (token) {
		headers['Authorization'] = `Bearer ${token}`;
	}

	const res = await fetch(`${BASE}${path}`, { method: 'POST', headers, body: formData });

	if (res.status === 401) {
		await auth.logout();
		throw new Error('Session expired');
	}

	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new Error(body.detail || body.title || `Request failed: ${res.status}`);
	}

	return res.json();
}

async function download(path: string): Promise<void> {
	const token = await auth.getIdToken();
	const headers: Record<string, string> = {};
	if (token) {
		headers['Authorization'] = `Bearer ${token}`;
	}

	const res = await fetch(`${BASE}${path}`, { headers });

	if (res.status === 401) {
		await auth.logout();
		throw new Error('Session expired');
	}

	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new Error(body.detail || body.title || `Request failed: ${res.status}`);
	}

	const disposition = res.headers.get('Content-Disposition') ?? '';
	const match = disposition.match(/filename="?([^"]+)"?/);
	const filename = match ? match[1] : path.split('/').pop() || 'download';

	const blob = await res.blob();
	const url = URL.createObjectURL(blob);
	const a = document.createElement('a');
	a.href = url;
	a.download = filename;
	document.body.appendChild(a);
	a.click();
	a.remove();
	URL.revokeObjectURL(url);
}

export const api = {
	get: <T>(path: string) => request<T>(path),
	list: <T>(path: string) => listAll<T>(path),
	post: <T>(path: string, body?: unknown) =>
		request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
	put: <T>(path: string, body?: unknown) =>
		request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined }),
	del: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
	upload: <T>(path: string, formData: FormData) => upload<T>(path, formData),
	download: (path: string) => download(path)
};
