import { describe, it, expect } from 'vitest';
import { canSeeHidden, isHidden, isLegHidden, publishLegsQuestion } from './publication.js';

const published = { published_at: '2026-01-01T00:00:00Z' };
const draft = {};

describe('canSeeHidden', () => {
	it.each([
		['admin', { isAdmin: true, user: { is_captain: false } }, true],
		['captain group', { isAdmin: false, user: { is_captain: true } }, true],
		['member', { isAdmin: false, user: { is_captain: false } }, false],
		['profile not loaded', { isAdmin: false, user: null }, false]
	])('%s', (_name, auth, want) => {
		expect(canSeeHidden(auth)).toBe(want);
	});
});

describe('isHidden', () => {
	it('is true without a publication time', () => {
		expect(isHidden(draft)).toBe(true);
		expect(isHidden({ published_at: null })).toBe(true);
		expect(isHidden(published)).toBe(false);
	});
});

describe('isLegHidden', () => {
	it.each([
		['published leg in a published cruise', published, published, false],
		['hidden leg', draft, published, true],
		['published leg in a hidden cruise', published, draft, true],
		['published leg, cruise not loaded', published, null, false]
	])('%s', (_name, leg, cruise, want) => {
		expect(isLegHidden(leg, cruise)).toBe(want);
	});
});

describe('publishLegsQuestion', () => {
	it.each([
		[1, 'Opublikować też 1 ukryty etap?'],
		[3, 'Opublikować też 3 ukryte etapy?'],
		[5, 'Opublikować też 5 ukrytych etapów?']
	])('%i legs', (n, want) => {
		expect(publishLegsQuestion(n)).toBe(want);
	});
});
