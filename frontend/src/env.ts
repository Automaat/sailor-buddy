import { defineEnvVars } from '@sveltejs/kit/env';

// Unset variables fall back to '' so callers keep their `||` defaults.
export const variables = defineEnvVars({
	PUBLIC_FIREBASE_API_KEY: { public: true, schema: (input) => input ?? '' },
	PUBLIC_FIREBASE_AUTH_DOMAIN: { public: true, schema: (input) => input ?? '' },
	PUBLIC_FIREBASE_PROJECT_ID: { public: true, schema: (input) => input ?? '' },
	PUBLIC_FIREBASE_AUTH_EMULATOR_URL: { public: true, schema: (input) => input ?? '' },
	PUBLIC_API_URL: { public: true, schema: (input) => input ?? '' }
});
