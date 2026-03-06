import { onAuthStateChanged, type User as FirebaseUser } from 'firebase/auth';
import { firebaseAuth } from '$lib/firebase';
import type { User } from '$lib/api/types';

function createAuthStore() {
	let firebaseUser = $state<FirebaseUser | null>(null);
	let dbUser = $state<User | null>(null);
	let loading = $state(true);

	if (typeof window !== 'undefined') {
		onAuthStateChanged(firebaseAuth, (user) => {
			firebaseUser = user;
			if (!user) {
				dbUser = null;
			}
			loading = false;
		});
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
		get loading() {
			return loading;
		},
		async getIdToken(): Promise<string | null> {
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
