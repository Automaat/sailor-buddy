import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: './e2e',
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
