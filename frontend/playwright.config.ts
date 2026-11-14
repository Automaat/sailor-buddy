import { defineConfig } from '@playwright/test';

// One ID per run, inherited by every worker, so specs share the suite admin
// (see SUITE_ADMIN in e2e/helpers.ts).
process.env.E2E_SUITE_ID ||= Date.now().toString(36);

export default defineConfig({
	testDir: './e2e',
	globalSetup: './e2e/global-setup.ts',
	timeout: 30_000,
	retries: 1,
	workers: 1,
	use: {
		baseURL: process.env.BASE_URL || 'http://localhost:5173',
		headless: true,
		screenshot: 'only-on-failure',
		trace: 'retain-on-failure'
	},
	projects: [
		{
			name: 'chromium',
			use: { browserName: 'chromium' }
		}
	]
});
