import { describe, it, expect } from 'vitest';
import {
	dayCount,
	daysLabel,
	formatDate,
	formatDateRange,
	formatDateRangeWithDays,
	formatTimestampDate
} from './dates.js';

describe('formatDate', () => {
	it.each([
		['2026-08-01', '1 sie 2026'],
		['2026-05-10', '10 maj 2026'],
		['2026-10-31', '31 paź 2026'],
		['2026-08-01T00:00:00Z', '1 sie 2026']
	])('renders %s as %s', (value, want) => {
		expect(formatDate(value)).toBe(want);
	});

	it('is empty without a date', () => {
		expect(formatDate(undefined)).toBe('');
		expect(formatDate(null)).toBe('');
		expect(formatDate('')).toBe('');
	});

	it.each(['soon', '2026-02-30', '2026-13-01'])('returns the invalid value %s unchanged', (value) => {
		expect(formatDate(value)).toBe(value);
	});
});

describe('formatDateRange', () => {
	it.each([
		['same month', '2026-08-01', '2026-08-09', '1–9 sie 2026'],
		['cross month', '2026-02-28', '2026-03-03', '28 lut – 3 mar 2026'],
		['cross year', '2026-12-30', '2027-01-04', '30 gru 2026 – 4 sty 2027'],
		['one day', '2026-08-01', '2026-08-01', '1 sie 2026'],
		['end before start', '2026-08-09', '2026-08-01', '9 sie 2026 – 1 sie 2026']
	])('renders a %s range', (_, start, end, want) => {
		expect(formatDateRange(start, end)).toBe(want);
	});

	it('renders the start alone without an end date', () => {
		expect(formatDateRange('2026-08-01', undefined)).toBe('1 sie 2026');
		expect(formatDateRange('2026-08-01', null)).toBe('1 sie 2026');
		expect(formatDateRange('2026-08-01', '')).toBe('1 sie 2026');
	});

	it('renders the end alone without a start date', () => {
		expect(formatDateRange(undefined, '2026-08-09')).toBe('9 sie 2026');
	});

	it('shows an invalid date unchanged next to a valid one', () => {
		expect(formatDateRange('2026-08-01', 'later')).toBe('1 sie 2026 – later');
		expect(formatDateRange('soon', '2026-08-09')).toBe('soon – 9 sie 2026');
	});
});

describe('dayCount', () => {
	it.each([
		['2026-08-01', '2026-08-09', 9],
		['2026-08-01', '2026-08-01', 1],
		['2026-02-28', '2026-03-03', 4],
		['2026-12-30', '2027-01-04', 6],
		['2026-03-28', '2026-03-30', 3]
	])('counts %s to %s as %i days', (start, end, want) => {
		expect(dayCount(start, end)).toBe(want);
	});

	it.each([
		['2026-08-01', undefined],
		[undefined, '2026-08-01'],
		['2026-08-01', 'later'],
		['2026-08-09', '2026-08-01']
	])('is null for %s to %s', (start, end) => {
		expect(dayCount(start, end)).toBeNull();
	});
});

describe('daysLabel', () => {
	it.each([
		[0, '0 dni'],
		[1, '1 dzień'],
		[2, '2 dni'],
		[12, '12 dni'],
		[5, '5 dni'],
		[21, '21 dni'],
		[22, '22 dni']
	])('labels %i', (n, want) => {
		expect(daysLabel(n)).toBe(want);
	});
});

describe('formatDateRangeWithDays', () => {
	it('adds the number of days to a range', () => {
		expect(formatDateRangeWithDays('2026-08-01', '2026-08-09')).toBe('1–9 sie 2026 · 9 dni');
		expect(formatDateRangeWithDays('2026-08-01', '2026-08-01')).toBe('1 sie 2026 · 1 dzień');
	});

	it('leaves the length out when it is unknown', () => {
		expect(formatDateRangeWithDays('2026-08-01', undefined)).toBe('1 sie 2026');
		expect(formatDateRangeWithDays('2026-08-01', 'later')).toBe('1 sie 2026 – later');
	});
});

describe('formatTimestampDate', () => {
	it('renders the Warsaw calendar day of a timestamp', () => {
		expect(formatTimestampDate('2026-11-12T23:30:00Z')).toBe('13 lis 2026');
		expect(formatTimestampDate('2026-07-01T10:00:00Z')).toBe('1 lip 2026');
	});

	it('is empty without a value and returns an invalid value unchanged', () => {
		expect(formatTimestampDate(undefined)).toBe('');
		expect(formatTimestampDate('nope')).toBe('nope');
	});
});
