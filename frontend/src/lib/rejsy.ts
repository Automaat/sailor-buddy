import type { StandaloneLeg, Trip } from '#lib/api/types.js';
import { isLegHidden } from '#lib/publication.js';

/** The club's calendar date (YYYY-MM-DD) in Europe/Warsaw, whatever the device time zone. */
export function warsawToday(now: Date = new Date()): string {
	return new Intl.DateTimeFormat('en-CA', {
		timeZone: 'Europe/Warsaw',
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).format(now);
}

type LegDates = Pick<Trip, 'status' | 'disembark_date'>;

/**
 * True for a planned leg whose disembark date is before today: it sailed but
 * still waits to be completed.
 */
export function isLegOverdue(leg: LegDates, today: string): boolean {
	if (leg.status !== 'planned') return false;
	const end = (leg.disembark_date || '').slice(0, 10);
	return end !== '' && end < today;
}

export type LegStatus = 'planned' | 'overdue' | 'cancelled';

export const legStatusLabels: Record<LegStatus, string> = {
	planned: 'Nadchodzący',
	overdue: 'Do rozliczenia',
	cancelled: 'Anulowany'
};

export const legStatusBadges: Record<LegStatus, string> = {
	planned: 'bg-[var(--ocean-tint)] text-[var(--ocean)]',
	overdue: 'bg-[var(--navy)] text-white',
	cancelled: 'bg-gray-100 text-gray-500'
};

/** The status shown for a planned trip leg; derived on read, the stored status stays untouched. */
export function legStatus(leg: LegDates, today: string = warsawToday()): LegStatus {
	if (leg.status === 'cancelled') return 'cancelled';
	return isLegOverdue(leg, today) ? 'overdue' : 'planned';
}

export type StandaloneStatus = LegStatus | 'completed';

export const standaloneStatusLabels: Record<StandaloneStatus, string> = {
	...legStatusLabels,
	completed: 'Zrealizowany'
};

export const standaloneStatusBadges: Record<StandaloneStatus, string> = {
	...legStatusBadges,
	completed: 'bg-[var(--navy)]/10 text-[var(--navy)]'
};

/** standaloneHref opens a "Bez serii" entry's own page: its trip, its voyage, or a cruise without legs. */
export function standaloneHref(entry: Pick<StandaloneLeg, 'kind' | 'id'>): string {
	if (entry.kind === 'trip') return `/trips/${entry.id}`;
	if (entry.kind === 'voyage') return `/voyages/${entry.id}`;
	return `/cruises/${entry.id}`;
}

/** True while members cannot see the entry: it or its cruise is a draft. */
export function isStandaloneHidden(
	entry: Pick<StandaloneLeg, 'published_at' | 'cruise_published_at'>
): boolean {
	return isLegHidden(
		{ published_at: entry.published_at },
		{ published_at: entry.cruise_published_at }
	);
}

/** A voyage sailed; a trip is planned, overdue or cancelled; a cruise without legs has no status. */
export function standaloneStatus(
	entry: Pick<StandaloneLeg, 'kind' | 'cancelled' | 'disembark_date'>,
	today: string = warsawToday()
): StandaloneStatus | null {
	if (entry.kind === 'voyage') return 'completed';
	if (entry.kind === 'cruise') return null;
	const status = entry.cancelled ? 'cancelled' : 'planned';
	return legStatus({ status, disembark_date: entry.disembark_date }, today);
}
