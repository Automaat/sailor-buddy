import type { Cruise, Trip, Voyage } from '#lib/api/types.js';

export type RejsStatus = 'upcoming' | 'completed' | 'cancelled';
export type RejsTab = 'upcoming' | 'completed' | 'all';

export const rejsTabs: { value: RejsTab; label: string }[] = [
	{ value: 'upcoming', label: 'Nadchodzące' },
	{ value: 'completed', label: 'Zrealizowane' },
	{ value: 'all', label: 'Wszystkie' }
];

export const rejsStatusLabels: Record<RejsStatus, string> = {
	upcoming: 'Planowany',
	completed: 'Zrealizowany',
	cancelled: 'Anulowany'
};

export type RejsEntry = {
	key: string;
	href: string;
	name: string;
	status: RejsStatus;
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

export function parseRejsTab(value: string | null): RejsTab {
	return value === 'completed' || value === 'all' ? value : 'upcoming';
}

/** Completed once a leg sailed and none is still planned; cancelled when only cancelled legs remain. */
function cruiseStatus(planned: number, cancelled: number, completed: number): RejsStatus {
	if (completed > 0 && planned === 0) return 'completed';
	if (planned === 0 && completed === 0 && cancelled > 0) return 'cancelled';
	return 'upcoming';
}

/** Builds the "Rejsy" list: one row per club event, with its legs summarized; legs stay on the event page. */
export function buildRejsEntries(cruises: Cruise[], trips: Trip[], voyages: Voyage[]): RejsEntry[] {
	return cruises.map((c) => {
		const legs = trips.filter((t) => t.cruise_id === c.id);
		const planned = legs.filter((t) => t.status === 'planned').length;
		const cancelled = legs.length - planned;
		const sailed = voyages.filter((v) => v.cruise_id === c.id);
		const legDates = [...legs, ...sailed]
			.map((l) => l.embark_date)
			.filter((d): d is string => !!d)
			.sort();
		return {
			key: `cruise-${c.id}`,
			href: `/cruises/${c.id}`,
			name: c.name,
			status: cruiseStatus(planned, cancelled, sailed.length),
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

/** Upcoming sorts soonest first with undated last; completed and all sort newest first. */
export function filterRejsEntries(entries: RejsEntry[], tab: RejsTab): RejsEntry[] {
	const picked =
		tab === 'all'
			? [...entries]
			: entries.filter((e) => e.status === (tab === 'upcoming' ? 'upcoming' : 'completed'));
	const ascending = tab === 'upcoming';
	return picked.sort((a, b) => {
		if (!a.embarkDate && !b.embarkDate) return 0;
		if (!a.embarkDate) return 1;
		if (!b.embarkDate) return -1;
		const cmp = a.embarkDate.localeCompare(b.embarkDate);
		return ascending ? cmp : -cmp;
	});
}
