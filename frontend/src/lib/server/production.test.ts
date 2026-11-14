import { describe, expect, it } from 'vitest';
import { validateProductionConfig } from './production';

const production = {
	SAILOR_ENV: 'production',
	PUBLIC_FIREBASE_API_KEY: 'public-web-key',
	PUBLIC_FIREBASE_AUTH_DOMAIN: 'sailor-club.firebaseapp.com',
	PUBLIC_FIREBASE_PROJECT_ID: 'sailor-club'
};

describe('production configuration', () => {
	it('accepts real project configuration and same-origin API routing', () => {
		expect(() => validateProductionConfig(production)).not.toThrow();
		expect(() =>
			validateProductionConfig({ ...production, PUBLIC_API_URL: 'https://api.example.com/api' })
		).not.toThrow();
	});
	it.each([
		['SAILOR_ENV', 'prod'],
		['PUBLIC_FIREBASE_API_KEY', ''],
		['PUBLIC_FIREBASE_API_KEY', 'fake-api-key'],
		['PUBLIC_FIREBASE_PROJECT_ID', ''],
		['PUBLIC_FIREBASE_PROJECT_ID', 'sailor-buddy-dev'],
		['PUBLIC_FIREBASE_PROJECT_ID', 'demo-club'],
		['PUBLIC_FIREBASE_AUTH_DOMAIN', ''],
		['PUBLIC_FIREBASE_AUTH_DOMAIN', 'localhost'],
		['PUBLIC_FIREBASE_AUTH_DOMAIN', 'https://club.firebaseapp.com'],
		['PUBLIC_FIREBASE_AUTH_EMULATOR_URL', 'http://localhost:9099'],
		['VITE_FIREBASE_AUTH_EMULATOR_URL', 'http://localhost:9099'],
		['PUBLIC_API_URL', 'http://api.example.com/api'],
		['PUBLIC_API_URL', 'https://user:pass@api.example.com/api'],
		['PUBLIC_API_URL', 'https://api.example.com/api?token=x'],
		['PUBLIC_API_URL', 'https://api.example.com/api#x'],
		['PUBLIC_API_URL', 'https://api.example.com/other']
	])('rejects unsafe %s=%s before serving requests', (key, value) => {
		expect(() => validateProductionConfig({ ...production, [key]: value })).toThrow();
	});
	it('rejects a baked emulator setting even with valid runtime settings', () => {
		expect(() => validateProductionConfig(production, 'http://localhost:9099')).toThrow();
	});
	it('supports an explicit development emulator', () => {
		expect(() =>
			validateProductionConfig({
				SAILOR_ENV: 'development',
				PUBLIC_FIREBASE_AUTH_EMULATOR_URL: 'http://localhost:9099'
			})
		).not.toThrow();
	});
});
