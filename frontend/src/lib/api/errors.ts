// errorMessage extracts a displayable message from an unknown thrown value.
// The API client (client.ts) always rejects with Error instances; anything
// else falls back to a string form so callers never surface
// `[object Object]` or `undefined` to the user.
export function errorMessage(e: unknown): string {
	if (e instanceof Error) return e.message;
	if (typeof e === 'string') return e;
	return 'Something went wrong';
}
