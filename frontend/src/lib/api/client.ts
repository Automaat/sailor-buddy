import { auth } from '$lib/stores/auth';
import type { AuthResponse } from './types';

const BASE = '/api';

async function request<T>(path: string, opts: RequestInit = {}): Promise<T> {
	const token = auth.getAccessToken();
	const headers: Record<string, string> = {
		'Content-Type': 'application/json',
		...((opts.headers as Record<string, string>) || {})
	};
	if (token) {
		headers['Authorization'] = `Bearer ${token}`;
	}

	let res = await fetch(`${BASE}${path}`, { ...opts, headers });

	if (res.status === 401 && token) {
		const refreshed = await refreshTokens();
		if (refreshed) {
			headers['Authorization'] = `Bearer ${auth.getAccessToken()}`;
			res = await fetch(`${BASE}${path}`, { ...opts, headers });
		} else {
			auth.logout();
			throw new Error('Session expired');
		}
	}

	if (!res.ok) {
		const body = await res.json().catch(() => ({}));
		throw new Error(body.error || `Request failed: ${res.status}`);
	}

	if (res.status === 204) return undefined as T;
	return res.json();
}

async function refreshTokens(): Promise<boolean> {
	const refreshToken = auth.getRefreshToken();
	if (!refreshToken) return false;

	try {
		const res = await fetch(`${BASE}/auth/refresh`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ refresh_token: refreshToken })
		});
		if (!res.ok) return false;
		const data: AuthResponse = await res.json();
		auth.setTokens(data.access_token, data.refresh_token);
		return true;
	} catch {
		return false;
	}
}

export const api = {
	get: <T>(path: string) => request<T>(path),
	post: <T>(path: string, body?: unknown) =>
		request<T>(path, { method: 'POST', body: body ? JSON.stringify(body) : undefined }),
	put: <T>(path: string, body?: unknown) =>
		request<T>(path, { method: 'PUT', body: body ? JSON.stringify(body) : undefined }),
	del: <T>(path: string) => request<T>(path, { method: 'DELETE' })
};
