import { listCountries } from '#lib/api/routes.js';
import type { Country } from '#lib/api/types.js';

export type { Country };

let cached: Promise<Country[]> | null = null;

/**
 * loadCountries resolves to the country list the API serves (GET
 * /countries): the one source of codes and Polish names. It is fetched once
 * per session; a failed fetch is retried on the next call.
 */
export function loadCountries(): Promise<Country[]> {
	cached ??= Promise.resolve()
		.then(listCountries)
		.catch((e: unknown) => {
			cached = null;
			throw e;
		});
	return cached;
}

const names = new WeakMap<readonly Country[], Map<string, string>>();

function nameIndex(countries: readonly Country[]): Map<string, string> {
	let index = names.get(countries);
	if (!index) {
		index = new Map(countries.map((c) => [c.code, c.name]));
		names.set(countries, index);
	}
	return index;
}

/** countryName is the Polish name of code, or code itself when the list lacks it. */
export function countryName(code: string, countries: readonly Country[]): string {
	return nameIndex(countries).get(code) ?? code;
}

/** formatCountries joins the Polish names of codes with ", ", in their order. */
export function formatCountries(
	codes: readonly string[] | null | undefined,
	countries: readonly Country[]
): string {
	return (codes ?? []).map((code) => countryName(code, countries)).join(', ');
}

const collator = new Intl.Collator('pl');

/** fold lower-cases s and drops its accents, so "lot" finds "Łotwa". */
function fold(s: string): string {
	return s
		.normalize('NFD')
		.replace(/\p{M}/gu, '')
		.replace(/ł/g, 'l')
		.replace(/Ł/g, 'L')
		.toLowerCase();
}

/**
 * searchCountries lists the countries whose Polish name contains query,
 * ignoring case and accents, or whose code is query. Names that start with
 * it come first; each group is in Polish alphabetical order. A blank query
 * lists every country.
 */
export function searchCountries(countries: readonly Country[], query: string): Country[] {
	const q = fold(query.trim());
	const byName = [...countries].sort((a, b) => collator.compare(a.name, b.name));
	if (!q) return byName;
	const starts: Country[] = [];
	const contains: Country[] = [];
	for (const c of byName) {
		const name = fold(c.name);
		if (name.startsWith(q) || fold(c.code) === q) starts.push(c);
		else if (name.includes(q)) contains.push(c);
	}
	return [...starts, ...contains];
}
