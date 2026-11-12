import type { Cruise } from '#lib/api/types.js';

export type CruiseKind = Cruise['kind'];

export const cruiseKinds: readonly CruiseKind[] = ['sea', 'inland'];

export const cruiseKindLabels: Record<CruiseKind, string> = {
	sea: 'Morski',
	inland: 'Śródlądowy'
};
