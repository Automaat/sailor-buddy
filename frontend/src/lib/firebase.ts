import { initializeApp } from 'firebase/app';
import { getAuth, connectAuthEmulator } from 'firebase/auth';

const firebaseConfig = {
	apiKey: import.meta.env.VITE_FIREBASE_API_KEY || 'fake-api-key',
	authDomain: import.meta.env.VITE_FIREBASE_AUTH_DOMAIN || 'localhost',
	projectId: import.meta.env.VITE_FIREBASE_PROJECT_ID || 'sailor-buddy-dev'
};

const app = initializeApp(firebaseConfig);
export const firebaseAuth = getAuth(app);

if (import.meta.env.DEV) {
	connectAuthEmulator(firebaseAuth, 'http://localhost:9099');
}
