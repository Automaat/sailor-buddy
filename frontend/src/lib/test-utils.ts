// Reusable factories and helpers shared across form/component test suites.
import type { CrewMember, Cruise, CruiseSeries, Member, Trip, Voyage } from '#lib/api/types.js';

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
		approved: true,
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

export function makeTrip(overrides: Partial<Trip> = {}): Trip {
	return {
		id: 1,
		cruise_id: 1,
		name: 'Rejs Bałtyk',
		status: 'planned',
		currency: 'PLN',
		embark_date: '2025-06-01',
		disembark_date: '2025-06-07',
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

export function makeVoyage(overrides: Partial<Voyage> = {}): Voyage {
	return {
		id: 1,
		cruise_id: 1,
		name: 'Rejs Bałtyk',
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
