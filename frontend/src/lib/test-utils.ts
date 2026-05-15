// Reusable factories and helpers shared across form/component test suites.
import type { Organization, Trip, Voyage } from '$lib/api/types';

export function makeOrg(overrides: Partial<Organization> = {}): Organization {
	return {
		id: 1,
		name: 'Klub Alfa',
		slug: 'alfa',
		role: 'admin',
		description: 'Opis klubu',
		city: 'Gdynia',
		website: 'https://alfa.example',
		pzz_club_number: 'PZZ-001',
		logo_url: '',
		created_at: '2026-01-01T00:00:00Z',
		updated_at: '2026-01-01T00:00:00Z',
		...overrides
	};
}

export function makeTrip(overrides: Partial<Trip> = {}): Trip {
	return {
		id: 1,
		owner_id: 1,
		name: 'Rejs Bałtyk',
		status: 'planned',
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
		owner_id: 1,
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
