import type { CruiseBody, CruiseSeries } from '#lib/api/types.js';

/** A series' kind; every edition of the series shares it. */
export type CruiseKind = CruiseSeries['kind'];

export const cruiseKinds: readonly CruiseKind[] = ['sea', 'inland'];

export const cruiseKindLabels: Record<CruiseKind, string> = {
	sea: 'Morski',
	inland: 'Śródlądowy'
};

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
