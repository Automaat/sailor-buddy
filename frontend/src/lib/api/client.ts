import { auth } from '$lib/stores/auth.svelte';

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
		throw new Error(body.error || `Request failed: ${res.status}`);
	}

	if (res.status === 204) return undefined as T;
	return res.json();
}

export const api = {
	get: <T>(path: string) => request<T>(path),
	post: <T>(path: string, body?: unknown) =>
		request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
	put: <T>(path: string, body?: unknown) =>
		request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined }),
	del: <T>(path: string) => request<T>(path, { method: 'DELETE' })
};
