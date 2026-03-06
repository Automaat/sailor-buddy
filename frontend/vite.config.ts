import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig, loadEnv } from 'vite';

export default defineConfig(({ mode }) => {
	const env = loadEnv(mode, process.cwd(), 'VITE_');
	return {
		plugins: [sveltekit(), tailwindcss()],
		server: {
			hmr: {
				clientPort: 5173
			},
			proxy: {
				'/api': env.VITE_BACKEND_URL || 'http://localhost:8080'
			}
		}
	};
});
