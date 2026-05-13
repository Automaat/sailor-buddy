import { initializeApp } from 'firebase/app';
import { getAuth, connectAuthEmulator } from 'firebase/auth';
import { env } from '$env/dynamic/public';

const firebaseConfig = {
	apiKey: env.PUBLIC_FIREBASE_API_KEY || import.meta.env.VITE_FIREBASE_API_KEY || 'fake-api-key',
	authDomain: env.PUBLIC_FIREBASE_AUTH_DOMAIN || import.meta.env.VITE_FIREBASE_AUTH_DOMAIN || 'localhost',
	projectId: env.PUBLIC_FIREBASE_PROJECT_ID || import.meta.env.VITE_FIREBASE_PROJECT_ID || 'sailor-buddy-dev'
};

const app = initializeApp(firebaseConfig);
export const firebaseAuth = getAuth(app);

if (import.meta.env.DEV) {
	const emulatorUrl =
		import.meta.env.VITE_FIREBASE_AUTH_EMULATOR_URL || 'http://localhost:9099';
	const authAny = firebaseAuth as any;

	// Guard against double-connecting the emulator (e.g. during HMR).
	if (authAny._canInitEmulator !== false) {
		connectAuthEmulator(firebaseAuth, emulatorUrl);
	}
}
