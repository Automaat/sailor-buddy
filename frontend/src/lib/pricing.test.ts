import { describe, it, expect } from 'vitest';
import {
	formatPrice,
	hasPriceInfo,
	listText,
	parseList,
	priceBody,
	priceFormFrom,
	priceLines,
	yourPrice
} from './pricing.js';

const twoTier = { currency: 'PLN' as const, cost_per_person: 820, cost_per_person_non_member: 870 };

describe('yourPrice', () => {
	it.each([
		['czlonek', 820],
		['sympatyk', 820],
		['gosc', 870]
	] as const)('gives %s the right tier', (membership, want) => {
		expect(yourPrice(twoTier, membership)).toBe(want);
	});

	it.each(['czlonek', 'sympatyk', 'gosc'] as const)('gives %s the only price when one is set', (membership) => {
		expect(yourPrice({ cost_per_person: 760 }, membership)).toBe(760);
		expect(yourPrice({ cost_per_person_non_member: 250 }, membership)).toBe(250);
	});

	it('treats a lone 0 as no price', () => {
		expect(yourPrice({ cost_per_person: 0 }, 'gosc')).toBeNull();
		expect(yourPrice({ cost_per_person_non_member: 0 }, 'czlonek')).toBeNull();
		expect(yourPrice({ cost_per_person: 0, cost_per_person_non_member: 0 }, 'czlonek')).toBeNull();
	});

	it('keeps a 0 tier next to a positive one', () => {
		const freeForMembers = { cost_per_person: 0, cost_per_person_non_member: 500 };
		expect(yourPrice(freeForMembers, 'czlonek')).toBe(0);
		expect(yourPrice(freeForMembers, 'sympatyk')).toBe(0);
		expect(yourPrice(freeForMembers, 'gosc')).toBe(500);
		expect(priceLines(freeForMembers).map((l) => l.value)).toEqual(['0 PLN', '500 PLN']);
		expect(hasPriceInfo(freeForMembers)).toBe(true);
	});

	it('returns null without membership when both tiers are set', () => {
		expect(yourPrice(twoTier, undefined)).toBeNull();
		expect(yourPrice({ cost_per_person: 500 }, undefined)).toBe(500);
	});
});

describe('priceLines', () => {
	it('lists both tiers, the yacht fund and the deposit in the cruise currency', () => {
		const lines = priceLines({ ...twoTier, currency: 'EUR', yacht_fund: 150, deposit: 12500 });
		expect(lines.map((l) => l.label)).toEqual([
			'Członkowie i sympatycy',
			'Goście',
			'Kasa jachtowa (szacunkowo)',
			'Kaucja (zwrotna)'
		]);
		expect(lines[2].value).toBe('150 EUR');
		expect(lines[3].value.replace(/\s/g, ' ')).toBe('12 500 EUR');
	});

	it('skips the tier lines when only one price is set', () => {
		expect(priceLines({ cost_per_person: 760 })).toEqual([]);
	});
});

describe('hasPriceInfo', () => {
	it('is false for an unpriced cruise with empty lists', () => {
		expect(hasPriceInfo({ currency: 'PLN', cost_per_person: 0 }, [], null)).toBe(false);
	});

	it('is true with a price or a list entry', () => {
		expect(hasPriceInfo({ deposit: 100 })).toBe(true);
		expect(hasPriceInfo({}, ['jacht'])).toBe(true);
		expect(hasPriceInfo({}, null, ['dojazd'])).toBe(true);
	});
});

describe('form helpers', () => {
	it('omits emptied amounts from the body and keeps zero', () => {
		const form = priceFormFrom({ currency: 'EUR', cost_per_person: 760 });
		form.deposit = 0;
		expect(priceBody(form)).toEqual({
			currency: 'EUR',
			cost_per_person: 760,
			cost_per_person_non_member: undefined,
			yacht_fund: undefined,
			deposit: 0
		});
	});

	it('defaults the currency to PLN', () => {
		expect(priceFormFrom().currency).toBe('PLN');
	});

	it('splits list text into trimmed, non-blank lines and back', () => {
		expect(parseList(' jacht \n\n  paliwo\r\n')).toEqual(['jacht', 'paliwo']);
		expect(listText(['jacht', 'paliwo'])).toBe('jacht\npaliwo');
		expect(listText(null)).toBe('');
	});

	it('formats amounts with Polish grouping', () => {
		expect(formatPrice(870, 'PLN')).toBe('870 PLN');
		expect(formatPrice(99.5, undefined)).toBe('99,5 PLN');
	});
});
