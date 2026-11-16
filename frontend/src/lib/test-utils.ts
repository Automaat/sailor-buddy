// Reusable factories and helpers shared across form/component test suites.
import { fireEvent, screen } from '@testing-library/svelte';
import type {
	Country,
	CrewMember,
	Cruise,
	CruiseSeries,
	Member,
	TripDetail,
	VoyageDetail
} from '#lib/api/types.js';

/** A few countries as GET /countries serves them, for pages that name countries. */
export const testCountries: Country[] = [
	{ code: 'DK', name: 'Dania' },
	{ code: 'FI', name: 'Finlandia' },
	{ code: 'HR', name: 'Chorwacja' },
	{ code: 'IT', name: 'Włochy' },
	{ code: 'MT', name: 'Malta' },
	{ code: 'NO', name: 'Norwegia' },
	{ code: 'PL', name: 'Polska' },
	{ code: 'SE', name: 'Szwecja' }
];

const removeCountry = 'Usuń kraj: ';

/** pickedCountries lists the names a CountryPicker shows as picked, in order. */
export function pickedCountries(): string[] {
	return screen
		.queryAllByRole('button', { name: new RegExp(`^${removeCountry}`) })
		.map((b) => (b.getAttribute('aria-label') ?? '').slice(removeCountry.length));
}

/** pickCountry types query into the CountryPicker labelled label and picks the first match. */
export async function pickCountry(query: string, label = 'Kraje') {
	const input = screen.getByLabelText(label);
	await fireEvent.input(input, { target: { value: query } });
	await fireEvent.keyDown(input, { key: 'Enter' });
}

export function makePerson(overrides: Partial<CrewMember> = {}): CrewMember {
	return {
		id: 1,
		full_name: 'Jan Kowalski',
		other_names: [],
		has_account: false,
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

export function makeMember(overrides: Partial<Member> = {}): Member {
	return {
		id: 1,
		name: 'Kasia Admin',
		suspended: false,
		email: 'kasia@example.dev',
		role: 'admin',
		is_captain: false,
		membership: 'czlonek',
		onboarded: true,
		created_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

export function makeCruise(overrides: Partial<Cruise> = {}): Cruise {
	return {
		id: 1,
		name: 'Regaty Klubowe',
		countries: [],
		included: [],
		not_included: [],
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

export function makeSeries(overrides: Partial<CruiseSeries> = {}): CruiseSeries {
	return {
		id: 1,
		name: 'Sycylia',
		kind: 'sea',
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

export function makeTrip(overrides: Partial<TripDetail> = {}): TripDetail {
	return {
		id: 1,
		cruise_id: 1,
		name: 'Rejs Bałtyk',
		status: 'planned',
		countries: [],
		currency: 'PLN',
		embark_date: '2025-06-01',
		disembark_date: '2025-06-07',
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

export function makeVoyage(overrides: Partial<VoyageDetail> = {}): VoyageDetail {
	return {
		id: 1,
		cruise_id: 1,
		name: 'Rejs Bałtyk',
		countries: [],
		hours_total: 0,
		hours_sail: 0,
		hours_engine: 0,
		hours_over_6bf: 0,
		miles: 0,
		days: 0,
		tidal_waters: 0,
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

// jsonResponse builds a minimal fetch Response stub for mocking globalThis.fetch.
export function jsonResponse(body: unknown, init: { status?: number } = {}): Response {
	const status = init.status ?? 200;
	return {
		ok: status >= 200 && status < 300,
		status,
		json: async () => body,
		headers: new Headers()
	} as Response;
}

// pagedListServer fakes a keyset-paged list endpoint over rows kept in id
// order: `after` resumes after that id, else `offset` skips rows. remove()
// deletes a row between two page requests, as another user's delete would.
export function pagedListServer<T extends { id: number }>(rows: T[]) {
	let live = [...rows];
	return {
		respond(url: string): Response {
			const params = new URL(url, 'http://test').searchParams;
			const limit = Number(params.get('limit') ?? 50);
			const after = params.get('after');
			let start = Number(params.get('offset') ?? 0);
			if (after !== null) {
				const i = live.findIndex((r) => r.id > Number(after));
				start = i === -1 ? live.length : i;
			}
			const items = live.slice(start, start + limit);
			const hasMore = start + limit < live.length;
			return jsonResponse({
				items,
				total: live.length,
				limit,
				offset: start,
				has_more: hasMore,
				...(hasMore ? { next_cursor: String(items[items.length - 1].id) } : {})
			});
		},
		remove(id: number) {
			live = live.filter((r) => r.id !== id);
		},
		ids(): number[] {
			return live.map((r) => r.id);
		}
	};
}

// unprefixedSpans lists the children of one-column grids that span several
// columns at every width: such a span adds implicit columns on phones and
// squeezes the other fields into a sliver.
export function unprefixedSpans(container: HTMLElement): string[] {
	const spans: string[] = [];
	for (const grid of container.querySelectorAll('.grid.grid-cols-1')) {
		for (const child of grid.children) {
			const span = [...child.classList].find((c) => /^col-span-(?!full$)/.test(c));
			if (span) spans.push(`${child.tagName.toLowerCase()}.${span}`);
		}
	}
	return spans;
}
