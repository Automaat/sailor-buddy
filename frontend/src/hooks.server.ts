import { validateProductionConfig } from '#lib/server/production.js';

export function init(): void {
	validateProductionConfig(process.env, import.meta.env.VITE_FIREBASE_AUTH_EMULATOR_URL);
}
