import type { Port } from '#lib/api/types.js';

/**
 * PortDraft is a port field's value: the name as typed and the coordinates
 * picked for it. A name typed without picking a place has none.
 */
export type PortDraft = { name: string; latitude?: number; longitude?: number };

/** A port whose position is known, so a map can draw it. */
export type LocatedPort = PortDraft & { latitude: number; longitude: number };

/** hasCoordinates reports whether a port carries both coordinates. */
export function hasCoordinates(
	p: { latitude?: number | null; longitude?: number | null } | null | undefined
): p is { latitude: number; longitude: number } {
	return typeof p?.latitude === 'number' && typeof p?.longitude === 'number';
}

/** portDraft starts a port field from a stored port, or empty without one. */
export function portDraft(p?: Port | null): PortDraft {
	if (!p) return { name: '' };
	return hasCoordinates(p)
		? { name: p.name, latitude: p.latitude, longitude: p.longitude }
		: { name: p.name };
}

/** normalizePortName trims a name and collapses its inner runs of blanks, as the server stores it. */
export function normalizePortName(name: string): string {
	return name.trim().replace(/\s+/g, ' ');
}

/** samePortName reports whether two typed names are the same port name once normalized. */
export function samePortName(a: string, b: string): boolean {
	return normalizePortName(a) === normalizePortName(b);
}

/**
 * portBody is a port field's value as a request sends it: undefined for an
 * empty field, so the port is omitted rather than sent as "".
 */
export function portBody(d: PortDraft): Port | undefined {
	const name = normalizePortName(d.name);
	if (!name) return undefined;
	return hasCoordinates(d) ? { name, latitude: d.latitude, longitude: d.longitude } : { name };
}
