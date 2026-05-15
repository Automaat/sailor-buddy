import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), 'VITE_');
	return {
		plugins: [sveltekit(), tailwindcss()],
		// Under vitest, resolve Svelte's client build so components mount in jsdom.
		resolve: process.env.VITEST ? { conditions: ['browser'] } : undefined,
		server: {
			hmr: {
				clientPort: 5173
			},
			proxy: {
				'/api': env.VITE_BACKEND_URL || 'http://localhost:8080'
			}
		},
		test: {
			environment: 'jsdom',
			globals: true,
			setupFiles: ['./src/vitest-setup.ts'],
			exclude: ['**/node_modules/**', '**/e2e/**'],
			coverage: {
				provider: 'v8',
				reporter: ['text', 'cobertura'],
				// Per-form gate: high-risk forms must stay above 80% so PRs
				// regressing their coverage fail CI.
				thresholds: {
					'src/lib/components/CompleteTripModal.svelte': {
						statements: 80,
						branches: 80,
						functions: 80,
						lines: 80
					},
					'src/routes/import/+page.svelte': {
						statements: 80,
						branches: 80,
						functions: 80,
						lines: 80
					},
					'src/routes/orgs/**/settings/+page.svelte': {
						statements: 80,
						branches: 80,
						functions: 80,
						lines: 80
					}
				}
			}
		}
	};
});
