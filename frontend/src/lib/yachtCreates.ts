import type { Yacht } from '#lib/api/types.js';

// Creates still waiting for the server, shared by every yacht picker on the
// page, so a draft re-added after a cancel or remount reuses the pending
// request instead of saving the same yacht twice. Plain module state: no UI
// reads it reactively.
const pendingCreates = new Map<string, Promise<Yacht>>();

export function pendingKey(name: string, type: string) {
	return JSON.stringify([type.toLocaleLowerCase(), name.toLocaleLowerCase()]);
}

export function createOnce(key: string, start: () => Promise<Yacht>) {
	const pending = pendingCreates.get(key);
	if (pending) return pending;
	const request = start();
	pendingCreates.set(key, request);
	const forget = () => {
		if (pendingCreates.get(key) === request) pendingCreates.delete(key);
	};
	request.then(forget, forget);
	return request;
}
