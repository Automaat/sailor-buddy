// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
	namespace App {
		// Error is what a load function throws with error(); +error.svelte
		// renders it. title and the back link let a 404 name what is missing.
		interface Error {
			message: string;
			title?: string;
			backHref?: string;
			backLabel?: string;
		}
		// interface Locals {}
		// interface PageData {}
		// interface PageState {}
		// interface Platform {}
	}
}

export {};
