import { initializeApp } from 'firebase/app';
import { getAuth, connectAuthEmulator, type Auth } from 'firebase/auth';
import { env } from '$env/dynamic/public';

const firebaseConfig = {
	apiKey: env.PUBLIC_FIREBASE_API_KEY || import.meta.env.VITE_FIREBASE_API_KEY || 'fake-api-key',
	authDomain: env.PUBLIC_FIREBASE_AUTH_DOMAIN || import.meta.env.VITE_FIREBASE_AUTH_DOMAIN || 'localhost',
	projectId: env.PUBLIC_FIREBASE_PROJECT_ID || import.meta.env.VITE_FIREBASE_PROJECT_ID || 'sailor-buddy-dev'
};

const app = initializeApp(firebaseConfig);
export const firebaseAuth = getAuth(app);

// connectEmulator points an Auth instance at the local emulator. A second
// call — e.g. when a Vite HMR reload re-runs this module — throws because the
// instance is already configured; that is expected, so it is swallowed.
// This relies on documented throwing behaviour rather than the undocumented
// internal `_canInitEmulator` flag, so it survives Firebase SDK upgrades.
export function connectEmulator(auth: Auth, url: string): void {
	try {
		connectAuthEmulator(auth, url);
	} catch (e) {
		console.debug('auth emulator already connected', e);
	}
}

// Connecting to the emulator is a runtime decision, not a build-time one: the
// production image is a single artifact run against the local emulator in
// docker-compose. PUBLIC_FIREBASE_AUTH_EMULATOR_URL is read at runtime via
// $env/dynamic/public; the import.meta.env paths cover the vite dev server.
const emulatorUrl =
	env.PUBLIC_FIREBASE_AUTH_EMULATOR_URL ||
	import.meta.env.VITE_FIREBASE_AUTH_EMULATOR_URL ||
	(import.meta.env.DEV ? 'http://localhost:9099' : undefined);

if (emulatorUrl) {
	connectEmulator(firebaseAuth, emulatorUrl);
}
