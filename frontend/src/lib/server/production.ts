export function validateProductionConfig(
	env: Record<string, string | undefined>,
	bakedEmulator?: string
): void {
	const mode = env.SAILOR_ENV || 'development';
	if (!['development', 'production'].includes(mode))
		throw new Error('SAILOR_ENV must be development or production');
	if (mode !== 'production') return;
	if (env.PUBLIC_FIREBASE_AUTH_EMULATOR_URL || env.VITE_FIREBASE_AUTH_EMULATOR_URL || bakedEmulator)
		throw new Error('Firebase emulator is forbidden in production');
	if (!env.PUBLIC_FIREBASE_API_KEY || env.PUBLIC_FIREBASE_API_KEY === 'fake-api-key')
		throw new Error('PUBLIC_FIREBASE_API_KEY is required in production');
	const project = env.PUBLIC_FIREBASE_PROJECT_ID;
	if (!project || project === 'sailor-buddy-dev' || project.startsWith('demo-'))
		throw new Error('Production Firebase project is required');
	const domain = env.PUBLIC_FIREBASE_AUTH_DOMAIN;
	if (!domain || domain === 'localhost' || /[/:@?#\\]/.test(domain))
		throw new Error('Production Firebase auth domain is required');
	const api = env.PUBLIC_API_URL || '/api';
	if (api !== '/api') {
		const url = new URL(api);
		if (
			url.protocol !== 'https:' ||
			url.username ||
			url.password ||
			url.search ||
			url.hash ||
			url.pathname !== '/api'
		)
			throw new Error('PUBLIC_API_URL must be /api or an HTTPS API base');
	}
}
