import { onAuthStateChanged, type User as FirebaseUser } from 'firebase/auth';
import { firebaseAuth } from '#lib/firebase.js';
import type { User } from '#lib/api/types.js';

function createAuthStore() {
	let firebaseUser = $state<FirebaseUser | null>(null);
	let dbUser = $state<User | null>(null);
	let loading = $state(true);

	// restored settles once Firebase reports the first auth state. A page load
	// function runs before the layout renders, so without it a direct visit
	// would request data before the saved session is restored and get a 401.
	let markRestored: () => void = () => {};
	const restored = new Promise<void>((resolve) => {
		markRestored = resolve;
	});

	if (typeof window !== 'undefined') {
		onAuthStateChanged(firebaseAuth, (user) => {
			firebaseUser = user;
			if (!user) {
				dbUser = null;
			}
			loading = false;
			markRestored();
		});
	} else {
		markRestored();
	}

	return {
		get user() {
			return dbUser;
		},
		set user(u: User | null) {
			dbUser = u;
		},
		get firebaseUser() {
			return firebaseUser;
		},
		get isAuthenticated() {
			return !!firebaseUser;
		},
		// isAdmin gates club-management UI. It reads the role from the /auth/me
		// profile, so it is false until that profile has loaded.
		get isAdmin() {
			return dbUser?.role === 'admin';
		},
		get loading() {
			return loading;
		},
		// getIdToken waits for the saved session to be restored, so every API
		// request, including one from a load function, carries the token.
		async getIdToken(): Promise<string | null> {
			await restored;
			if (!firebaseUser) return null;
			return firebaseUser.getIdToken();
		},
		async logout() {
			await firebaseAuth.signOut();
			firebaseUser = null;
			dbUser = null;
		}
	};
}

export const auth = createAuthStore();
