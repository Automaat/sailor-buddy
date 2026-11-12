import type { Cruise, User } from '#lib/api/types.js';

export type Currency = Cruise['currency'];
export type Membership = User['membership'];

export const currencies: readonly Currency[] = ['PLN', 'EUR'];

/** Priced is the price fields shared by cruises, trips and their enroll summaries. */
export interface Priced {
	currency?: Currency;
	cost_per_person?: number;
	cost_per_person_non_member?: number;
	yacht_fund?: number;
	deposit?: number;
}

/** amount treats 0 as "not set", because the forms store 0 for an empty amount. */
function amount(v: number | null | undefined): number | null {
	return v != null && v > 0 ? v : null;
}

/**
 * yourPrice is the berth price the caller pays. Members (czlonek) and
 * supporters (sympatyk) pay the member price, guests (gosc) the non-member
 * price. When only one price is set, everyone pays it. It returns null when
 * no price is set, or when both are set and the membership is unknown.
 */
export function yourPrice(p: Priced, membership: Membership | null | undefined): number | null {
	const member = amount(p.cost_per_person);
	const nonMember = amount(p.cost_per_person_non_member);
	if (member === null || nonMember === null) return member ?? nonMember;
	if (membership === 'gosc') return nonMember;
	if (membership === 'czlonek' || membership === 'sympatyk') return member;
	return null;
}

const amountFormat = new Intl.NumberFormat('pl-PL', { maximumFractionDigits: 2 });

/** formatPrice renders an amount with its currency, e.g. "12 500 PLN". */
export function formatPrice(value: number, currency: Currency | undefined): string {
	return `${amountFormat.format(value)} ${currency ?? 'PLN'}`;
}

export interface PriceLine {
	label: string;
	value: string;
}

/**
 * priceLines lists the price details shown under "Twoja cena": both tiers
 * when both are set, then the yacht fund and the deposit. Unset amounts are
 * skipped.
 */
export function priceLines(p: Priced): PriceLine[] {
	const lines: PriceLine[] = [];
	const member = amount(p.cost_per_person);
	const nonMember = amount(p.cost_per_person_non_member);
	const yachtFund = amount(p.yacht_fund);
	const deposit = amount(p.deposit);
	if (member !== null && nonMember !== null) {
		lines.push({ label: 'Członkowie i sympatycy', value: formatPrice(member, p.currency) });
		lines.push({ label: 'Goście', value: formatPrice(nonMember, p.currency) });
	}
	if (yachtFund !== null) {
		lines.push({ label: 'Kasa jachtowa (szacunkowo)', value: formatPrice(yachtFund, p.currency) });
	}
	if (deposit !== null) {
		lines.push({ label: 'Kaucja (zwrotna)', value: formatPrice(deposit, p.currency) });
	}
	return lines;
}

/**
 * hasPriceInfo reports whether a cruise or trip has anything to show in its
 * price section: an amount, or an "included" / "not included" entry.
 */
export function hasPriceInfo(
	p: Priced,
	included?: readonly string[] | null,
	notIncluded?: readonly string[] | null
): boolean {
	return (
		yourPrice(p, null) !== null ||
		priceLines(p).length > 0 ||
		(included?.length ?? 0) > 0 ||
		(notIncluded?.length ?? 0) > 0
	);
}

/** PriceForm is the editable price state of the cruise and trip forms. */
export interface PriceForm {
	currency: Currency;
	cost_per_person: number | null;
	cost_per_person_non_member: number | null;
	yacht_fund: number | null;
	deposit: number | null;
}

export function priceFormFrom(p: Priced = {}): PriceForm {
	return {
		currency: p.currency ?? 'PLN',
		cost_per_person: p.cost_per_person ?? null,
		cost_per_person_non_member: p.cost_per_person_non_member ?? null,
		yacht_fund: p.yacht_fund ?? null,
		deposit: p.deposit ?? null
	};
}

/** optional maps an emptied number input (null) to an omitted body field. */
function optional(v: number | null | undefined): number | undefined {
	return v == null || Number.isNaN(v) ? undefined : v;
}

/** priceBody maps the form state to the request body fields. */
export function priceBody(f: PriceForm) {
	return {
		currency: f.currency,
		cost_per_person: optional(f.cost_per_person),
		cost_per_person_non_member: optional(f.cost_per_person_non_member),
		yacht_fund: optional(f.yacht_fund),
		deposit: optional(f.deposit)
	};
}

/** parseList splits a textarea into list entries, one per non-blank line. */
export function parseList(text: string): string[] {
	return text
		.split('\n')
		.map((s) => s.trim())
		.filter((s) => s.length > 0);
}

/** listText joins list entries back into textarea text. */
export function listText(items: readonly string[] | null | undefined): string {
	return (items ?? []).join('\n');
}
