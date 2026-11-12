import type { Cruise, CruiseBody } from '#lib/api/types.js';

export type CruiseKind = Cruise['kind'];

export const cruiseKinds: readonly CruiseKind[] = ['sea', 'inland'];

export const cruiseKindLabels: Record<CruiseKind, string> = {
	sea: 'Morski',
	inland: 'Śródlądowy'
};

/** True for an absolute http(s) URL, the only links rendered as clickable. */
export function isHttpUrl(value: string): boolean {
	try {
		const u = new URL(value);
		return u.protocol === 'http:' || u.protocol === 'https:';
	} catch {
		return false;
	}
}

type CruiseForm = Omit<CruiseBody, 'series_id' | 'min_age'> & {
	series_id: number | null;
	min_age: number | null | undefined;
};

/**
 * Maps the cruise create/edit form to the request body. An empty number input
 * binds to null (or NaN), which the API must receive as an omitted field.
 */
export function cruiseBody(form: CruiseForm): CruiseBody {
	const minAge = form.min_age;
	return {
		...form,
		series_id: form.series_id ?? undefined,
		min_age: typeof minAge === 'number' && Number.isFinite(minAge) ? minAge : undefined
	};
}
