// errorMessage extracts a displayable message from an unknown thrown value.
// The API client (client.ts) always rejects with Error instances; anything
// else falls back to a string form so callers never surface
// `[object Object]` or `undefined` to the user.
export function errorMessage(e: unknown): string {
	if (e instanceof Error) return e.message;
	if (typeof e === 'string') return e;
	return 'Something went wrong';
}

// settle resolves to the value of p, or to fallback when p rejects. It logs
// the failure and hands it to onError, so the page can say which section did
// not load instead of showing the fallback (an empty list, a missing link) as
// if it were real. settle never rejects: one failed section does not take
// down a Promise.all over the others. It has no Svelte dependency, so a load
// function can use it the same way a component does.
export async function settle<T>(p: Promise<T>, fallback: T, onError: (e: unknown) => void): Promise<T> {
	try {
		return await p;
	} catch (e) {
		console.error('Load failed, showing fallback:', e);
		onError(e);
		return fallback;
	}
}

// noteFailure returns a settle onError that adds label to errors once. errors
// can be a $state array in a component or a plain array a load function
// returns; LoadErrorBanner renders either.
export function noteFailure(errors: string[], label: string): () => void {
	return () => {
		if (!errors.includes(label)) errors.push(label);
	};
}
