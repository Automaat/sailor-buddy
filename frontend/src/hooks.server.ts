import { validateProductionConfig } from '#lib/server/production.js';
import { secureResponse } from '#lib/server/security.js';
import {
	PUBLIC_API_URL,
	PUBLIC_FIREBASE_AUTH_DOMAIN,
	PUBLIC_FIREBASE_AUTH_EMULATOR_URL
} from '$app/env/public';
import type { Handle } from '@sveltejs/kit/hooks';

export function init(): void {
	validateProductionConfig(process.env, import.meta.env.VITE_FIREBASE_AUTH_EMULATOR_URL);
}

export const handle: Handle = async ({ event, resolve }) =>
	secureResponse(await resolve(event), event.url, {
		apiURL: PUBLIC_API_URL,
		authDomain:
			PUBLIC_FIREBASE_AUTH_DOMAIN || import.meta.env.VITE_FIREBASE_AUTH_DOMAIN || 'localhost',
		emulatorURL:
			PUBLIC_FIREBASE_AUTH_EMULATOR_URL ||
			import.meta.env.VITE_FIREBASE_AUTH_EMULATOR_URL ||
			(import.meta.env.DEV ? 'http://localhost:9099' : undefined),
		dev: import.meta.env.DEV
	});
