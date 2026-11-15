import { error } from '@sveltejs/kit';
import { ApiError } from './client.js';

// NotFoundCopy is what +error.svelte shows when the record a page loads is
// missing: the message, and optionally a title and a link back to its list.
export interface NotFoundCopy {
	message: string;
	title?: string;
	backHref?: string;
	backLabel?: string;
}

// found resolves to the record p loads, or throws error(404) with copy when
// the API answers 404 or returns null, so +error.svelte renders the not-found
// panel. Any other failure propagates to +error.svelte as a failed load.
export async function found<T>(p: Promise<T | null | undefined>, copy: NotFoundCopy): Promise<T> {
	let value: T | null | undefined;
	try {
		value = await p;
	} catch (e) {
		if (e instanceof ApiError && e.status === 404) notFound(copy);
		throw e;
	}
	if (value == null) notFound(copy);
	return value;
}

function notFound({ message, ...rest }: NotFoundCopy): never {
	error(404, message, rest);
}

/**
 * cruiseIdParam is the event a "new leg" link preselects with ?cruise_id=,
 * or 0 when the parameter is missing or not a positive integer.
 */
export function cruiseIdParam(url: URL): number {
	return idParam(url, 'cruise_id');
}

/**
 * seriesIdParam is the series a "new edition" link names with ?series_id=,
 * or 0 when the parameter is missing or not a positive integer.
 */
export function seriesIdParam(url: URL): number {
	return idParam(url, 'series_id');
}

function idParam(url: URL, name: string): number {
	const n = Number(url.searchParams.get(name) ?? 0);
	return Number.isInteger(n) && n > 0 ? n : 0;
}
