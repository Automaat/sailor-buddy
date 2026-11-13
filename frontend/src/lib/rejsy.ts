import type { Cruise, Trip, Voyage } from '#lib/api/types.js';

export type RejsStatus = 'upcoming' | 'overdue' | 'completed' | 'cancelled';
export type RejsTab = 'upcoming' | 'completed' | 'all';

export const rejsTabs: { value: RejsTab; label: string }[] = [
	{ value: 'upcoming', label: 'Nadchodzące' },
	{ value: 'completed', label: 'Zrealizowane' },
	{ value: 'all', label: 'Wszystkie' }
];

export const rejsStatusLabels: Record<RejsStatus, string> = {
	upcoming: 'Planowany',
	overdue: 'Do rozliczenia',
	completed: 'Zrealizowany',
	cancelled: 'Anulowany'
};

export type RejsEntry = {
	key: string;
	cruiseId: number;
	href: string;
	name: string;
	status: RejsStatus;
	/** True once the cruise's end (its own date, else its latest leg's) is before today. */
	ended: boolean;
	kind?: Cruise['kind'];
	embarkDate?: string;
	disembarkDate?: string;
	startPort?: string;
	endPort?: string;
	countries?: string;
	maxCrew?: number;
	enrollOpen: boolean;
	plannedLegs: number;
	completedLegs: number;
	hoursTotal: number;
	miles: number;
};

/** isMineFilter reads the "only my cruises" toggle from the ?mine= param. */
export function isMineFilter(value: string | null): boolean {
	return value === '1';
}

/** rejsListUrl keeps the Rejsy tab and the "only mine" toggle in the URL. */
export function rejsListUrl(tab: RejsTab, mine: boolean): string {
	const params = new URLSearchParams();
	if (tab !== 'upcoming') params.set('tab', tab);
	if (mine) params.set('mine', '1');
	const query = params.toString();
	return query ? `/cruises?${query}` : '/cruises';
}

/** onlyMine keeps the entries for cruises the user took part in. */
export function onlyMine(entries: RejsEntry[], myCruiseIds: ReadonlySet<number>): RejsEntry[] {
	return entries.filter((e) => myCruiseIds.has(e.cruiseId));
}

export function parseRejsTab(value: string | null): RejsTab {
	return value === 'completed' || value === 'all' ? value : 'upcoming';
}

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
 * True for a planned leg whose disembark date (or the cruise's, when the leg
 * has none) is before today: it sailed but still waits to be completed.
 */
export function isLegOverdue(
	leg: LegDates,
	cruise: Pick<Cruise, 'disembark_date'> | null | undefined,
	today: string
): boolean {
	if (leg.status !== 'planned') return false;
	const end = (leg.disembark_date || cruise?.disembark_date || '').slice(0, 10);
	return end !== '' && end < today;
}

export type LegStatus = 'planned' | 'overdue' | 'cancelled';

export const legStatusLabels: Record<LegStatus, string> = {
	planned: 'Planowany',
	overdue: 'Do rozliczenia',
	cancelled: 'Anulowany'
};

export const legStatusBadges: Record<LegStatus, string> = {
	planned: 'bg-blue-100 text-blue-700',
	overdue: 'bg-amber-100 text-amber-800',
	cancelled: 'bg-gray-100 text-gray-500'
};

/** The status shown for a planned trip leg; derived on read, the stored status stays untouched. */
export function legStatus(
	leg: LegDates,
	cruise: Pick<Cruise, 'disembark_date'> | null | undefined,
	today: string = warsawToday()
): LegStatus {
	if (leg.status === 'cancelled') return 'cancelled';
	return isLegOverdue(leg, cruise, today) ? 'overdue' : 'planned';
}

/**
 * Completed once a leg sailed and none is still planned; overdue when a planned
 * leg already ended; cancelled when only cancelled legs remain.
 */
function cruiseStatus(planned: number, overdue: number, cancelled: number, completed: number): RejsStatus {
	if (overdue > 0) return 'overdue';
	if (completed > 0 && planned === 0) return 'completed';
	if (planned === 0 && completed === 0 && cancelled > 0) return 'cancelled';
	return 'upcoming';
}

/** Builds the "Rejsy" list: one row per club event, with its legs summarized; legs stay on the event page. */
export function buildRejsEntries(
	cruises: Cruise[],
	trips: Trip[],
	voyages: Voyage[],
	today: string = warsawToday()
): RejsEntry[] {
	return cruises.map((c) => {
		const legs = trips.filter((t) => t.cruise_id === c.id);
		const planned = legs.filter((t) => t.status === 'planned').length;
		const overdue = legs.filter((t) => isLegOverdue(t, c, today)).length;
		const cancelled = legs.length - planned;
		const sailed = voyages.filter((v) => v.cruise_id === c.id);
		const legDates = [...legs, ...sailed]
			.map((l) => l.embark_date)
			.filter((d): d is string => !!d)
			.sort();
		const lastLegEnd = [...legs, ...sailed]
			.map((l) => (l.disembark_date || '').slice(0, 10))
			.filter((d) => d !== '')
			.sort()
			.at(-1);
		const end = (c.disembark_date || '').slice(0, 10) || lastLegEnd;
		return {
			key: `cruise-${c.id}`,
			cruiseId: c.id,
			href: `/cruises/${c.id}`,
			name: c.name,
			status: cruiseStatus(planned, overdue, cancelled, sailed.length),
			ended: !!end && end < today,
			kind: c.kind,
			embarkDate: c.embark_date || legDates[0],
			disembarkDate: c.disembark_date || undefined,
			startPort: c.start_port || undefined,
			endPort: c.end_port || undefined,
			countries: c.countries || undefined,
			maxCrew: c.max_crew || undefined,
			enrollOpen: !!c.enroll_token,
			plannedLegs: planned,
			completedLegs: sailed.length,
			hoursTotal: sailed.reduce((sum, v) => sum + v.hours_total, 0),
			miles: sailed.reduce((sum, v) => sum + v.miles, 0)
		};
	});
}

/**
 * A cruise awaiting completion lists as sailed once it has ended; while it is
 * still at sea (an earlier leg ended, a later one has not) it stays upcoming.
 */
function inTab(e: RejsEntry, tab: Exclude<RejsTab, 'all'>): boolean {
	if (e.status === 'overdue') return tab === (e.ended ? 'completed' : 'upcoming');
	return e.status === tab;
}

/** Upcoming sorts soonest first with undated last; completed and all sort newest first. */
export function filterRejsEntries(entries: RejsEntry[], tab: RejsTab): RejsEntry[] {
	const picked = tab === 'all' ? [...entries] : entries.filter((e) => inTab(e, tab));
	const ascending = tab === 'upcoming';
	return picked.sort((a, b) => {
		if (!a.embarkDate && !b.embarkDate) return 0;
		if (!a.embarkDate) return 1;
		if (!b.embarkDate) return -1;
		const cmp = a.embarkDate.localeCompare(b.embarkDate);
		return ascending ? cmp : -cmp;
	});
}
