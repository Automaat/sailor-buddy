import { describe, it, expect } from 'vitest';
import { myLegEditionLabel, myLegRelationBadge, myLegRelationLabel } from './myLegs';

describe('myLegRelationLabel', () => {
	it.each([
		[{ relation: 'captain' as const }, 'Kapitan'],
		[{ relation: 'crew' as const }, 'W załodze'],
		[{ relation: 'sign_up' as const, sign_up_status: 'pending' as const }, 'Zapis: oczekuje'],
		[{ relation: 'sign_up' as const, sign_up_status: 'accepted' as const }, 'Zapis: zaakceptowany'],
		[
			{ relation: 'sign_up' as const, sign_up_status: 'waitlisted' as const },
			'Zapis: lista rezerwowa'
		]
	])('labels %o as %s', (leg, label) => {
		expect(myLegRelationLabel(leg)).toBe(label);
	});
});

describe('myLegRelationBadge', () => {
	it('sets a confirmed place apart from a sign-up still in review', () => {
		const confirmed = myLegRelationBadge({ relation: 'crew' });
		expect(myLegRelationBadge({ relation: 'sign_up', sign_up_status: 'accepted' })).toBe(confirmed);
		expect(myLegRelationBadge({ relation: 'sign_up', sign_up_status: 'pending' })).not.toBe(
			confirmed
		);
		expect(myLegRelationBadge({ relation: 'captain' })).not.toBe(confirmed);
	});
});

describe('myLegEditionLabel', () => {
	it.each([
		[
			{ cruise_name: 'Sycylia 2027', series_name: 'Sycylia', edition: '2027' },
			'Sycylia · edycja 2027'
		],
		[{ cruise_name: 'Sycylia 2027', series_name: 'Sycylia' }, 'Sycylia'],
		[{ cruise_name: 'Bałtyk' }, 'Bałtyk']
	])('labels %o as %s', (leg, label) => {
		expect(myLegEditionLabel(leg)).toBe(label);
	});
});
