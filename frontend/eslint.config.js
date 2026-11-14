import js from '@eslint/js';
import prettier from 'eslint-config-prettier';
import svelte from 'eslint-plugin-svelte';
import { defineConfig } from 'eslint/config';
import globals from 'globals';
import ts from 'typescript-eslint';

export default defineConfig(
	{
		ignores: ['build/', '.svelte-kit/', 'coverage/', 'test-results/', 'playwright-report/']
	},
	js.configs.recommended,
	ts.configs.recommended,
	svelte.configs.recommended,
	prettier,
	svelte.configs.prettier,
	{
		languageOptions: {
			globals: { ...globals.browser, ...globals.node },
			parserOptions: {
				projectService: true,
				extraFileExtensions: ['.svelte'],
				tsconfigRootDir: import.meta.dirname
			}
		}
	},
	{
		files: ['**/*.svelte', '**/*.svelte.ts', '**/*.svelte.js'],
		languageOptions: {
			parserOptions: { parser: ts.parser }
		}
	},
	{
		rules: {
			'svelte/require-each-key': 'error',
			'@typescript-eslint/no-floating-promises': 'error',
			// No paths.base, so resolve() is a no-op. EXC:FILE011:deliberate-rule-choice
			'svelte/no-navigation-without-resolve': 'off'
		}
	},
	{
		files: ['**/*.js'],
		extends: [ts.configs.disableTypeChecked]
	}
);
