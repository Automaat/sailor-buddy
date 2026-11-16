/** Marks the missing end of a route with one port set. */
export const MISSING_PORT = '—';

type NamedPort = { name: string } | null | undefined;

/**
 * formatRoute renders a leg's route as "start → end". It returns null when
 * neither port is set; with one set, the other shows as MISSING_PORT.
 */
export function formatRoute(start: NamedPort, end: NamedPort): string | null {
	if (!start && !end) return null;
	return `${start?.name ?? MISSING_PORT} → ${end?.name ?? MISSING_PORT}`;
}
