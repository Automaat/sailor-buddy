import type { User } from '$lib/api/types';

interface AuthState {
	user: User | null;
	accessToken: string | null;
	refreshToken: string | null;
}

function createAuthStore() {
	let state = $state<AuthState>({
		user: null,
		accessToken: null,
		refreshToken: null
	});

	if (typeof window !== 'undefined') {
		const saved = localStorage.getItem('auth');
		if (saved) {
			try {
				const parsed = JSON.parse(saved);
				state.user = parsed.user;
				state.accessToken = parsed.accessToken;
				state.refreshToken = parsed.refreshToken;
			} catch {
				localStorage.removeItem('auth');
			}
		}
	}

	function persist() {
		if (typeof window !== 'undefined') {
			localStorage.setItem('auth', JSON.stringify(state));
		}
	}

	return {
		get user() {
			return state.user;
		},
		get isAuthenticated() {
			return !!state.accessToken;
		},
		getAccessToken() {
			return state.accessToken;
		},
		getRefreshToken() {
			return state.refreshToken;
		},
		setTokens(access: string, refresh: string) {
			state.accessToken = access;
			state.refreshToken = refresh;
			persist();
		},
		login(user: User, accessToken: string, refreshToken: string) {
			state.user = user;
			state.accessToken = accessToken;
			state.refreshToken = refreshToken;
			persist();
		},
		logout() {
			state.user = null;
			state.accessToken = null;
			state.refreshToken = null;
			if (typeof window !== 'undefined') {
				localStorage.removeItem('auth');
			}
		}
	};
}

export const auth = createAuthStore();
