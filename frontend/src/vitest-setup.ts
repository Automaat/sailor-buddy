import '@testing-library/jest-dom/vitest';
import { vi } from 'vitest';

// $app/env/public is a SvelteKit virtual module with no runtime value
// under vitest; stub it so modules importing it (api client, firebase) load.
vi.mock('$app/env/public', () => ({
	PUBLIC_FIREBASE_API_KEY: '',
	PUBLIC_FIREBASE_AUTH_DOMAIN: '',
	PUBLIC_FIREBASE_PROJECT_ID: '',
	PUBLIC_FIREBASE_AUTH_EMULATOR_URL: '',
	PUBLIC_API_URL: ''
}));
