/**
 * Polish date rendering for the UI. The API sends calendar dates as ISO
 * strings (YYYY-MM-DD); these helpers only change how they are shown, so
 * form inputs keep their native ISO values. Calendar dates are parsed and
 * formatted in UTC, so the browser's time zone never shifts them by a day.
 */

type DateValue = string | null | undefined;

const dateFormat = new Intl.DateTimeFormat('pl-PL', {
	day: 'numeric',
	month: 'short',
	year: 'numeric',
	timeZone: 'UTC'
});

const DAY_MS = 24 * 60 * 60 * 1000;

/**
 * parseDate reads the YYYY-MM-DD prefix of value. It returns null for
 * anything else, including dates that do not exist such as 2026-02-30,
 * which Date.UTC would silently roll over into March.
 */
function parseDate(value: DateValue): Date | null {
	const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(value ?? '');
	if (!m) return null;
	const [y, mo, d] = [Number(m[1]), Number(m[2]), Number(m[3])];
	const date = new Date(Date.UTC(y, mo - 1, d));
	const exists = date.getUTCFullYear() === y && date.getUTCMonth() === mo - 1 && date.getUTCDate() === d;
	return exists ? date : null;
}

interface DateParts {
	day: string;
	month: string;
	year: string;
}

function parts(date: Date): DateParts {
	const out: DateParts = { day: '', month: '', year: '' };
	for (const p of dateFormat.formatToParts(date)) {
		if (p.type === 'day' || p.type === 'month' || p.type === 'year') out[p.type] = p.value;
	}
	return out;
}

/**
 * formatDate renders a calendar date as "1 sie 2026". An empty value gives
 * "", and a value that is not a date is returned unchanged.
 */
export function formatDate(value: DateValue): string {
	const date = parseDate(value);
	return date ? dateFormat.format(date) : (value ?? '');
}

/**
 * formatDateRange renders a range as "1–9 sie 2026", "28 lut – 3 mar 2026"
 * or "30 gru 2026 – 4 sty 2027". With one date missing it renders the
 * other alone. Values that are not dates are shown unchanged.
 */
export function formatDateRange(start: DateValue, end: DateValue): string {
	if (!end) return formatDate(start);
	if (!start) return formatDate(end);
	const from = parseDate(start);
	const to = parseDate(end);
	if (!from || !to) return `${formatDate(start)} – ${formatDate(end)}`;
	if (from.getTime() === to.getTime()) return dateFormat.format(from);
	const a = parts(from);
	const b = parts(to);
	if (a.year !== b.year || to < from) return `${dateFormat.format(from)} – ${dateFormat.format(to)}`;
	if (a.month !== b.month) return `${a.day} ${a.month} – ${dateFormat.format(to)}`;
	return `${a.day}–${dateFormat.format(to)}`;
}

/**
 * dayCount is the number of days a range covers, counting both ends
 * (2026-08-01 to 2026-08-09 is 9 days). It is null when either date is
 * missing or invalid, or when the end is before the start.
 */
export function dayCount(start: DateValue, end: DateValue): number | null {
	const from = parseDate(start);
	const to = parseDate(end);
	if (!from || !to || to < from) return null;
	return Math.round((to.getTime() - from.getTime()) / DAY_MS) + 1;
}

/** daysLabel renders a day count with its Polish noun: "1 dzień", "9 dni". */
export function daysLabel(n: number): string {
	return `${n} ${n === 1 ? 'dzień' : 'dni'}`;
}

/**
 * formatDateRangeWithDays renders a range followed by its length, e.g.
 * "1–9 sie 2026 · 9 dni". The length is left out when it is unknown.
 */
export function formatDateRangeWithDays(start: DateValue, end: DateValue): string {
	const range = formatDateRange(start, end);
	const n = dayCount(start, end);
	return n === null ? range : `${range} · ${daysLabel(n)}`;
}

const warsawDateFormat = new Intl.DateTimeFormat('pl-PL', {
	day: 'numeric',
	month: 'short',
	year: 'numeric',
	timeZone: 'Europe/Warsaw'
});

/**
 * formatTimestampDate renders the club-calendar (Europe/Warsaw) day of a
 * timestamp, e.g. "13 lis 2026". A value that is not a timestamp is
 * returned unchanged.
 */
export function formatTimestampDate(value: DateValue): string {
	if (!value) return '';
	const t = Date.parse(value);
	return Number.isNaN(t) ? value : warsawDateFormat.format(new Date(t));
}
