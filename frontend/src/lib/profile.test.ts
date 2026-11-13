import { describe, it, expect } from 'vitest';
import type { User } from '#lib/api/types.js';
import { patentSummary, profileBody, profileFormFromUser, splitName } from './profile.js';

const user = (over: Partial<User> = {}): User =>
	({
		id: 1,
		email: 'jan@example.dev',
		name: 'Jan Kowalski',
		first_name: '',
		last_name: '',
		avatar_url: '',
		role: 'member',
		is_captain: false,
		membership: 'gosc',
		onboarded: false,
		...over
	}) as User;

describe('splitName', () => {
	it.each([
		['Jan Kowalski', 'Jan', 'Kowalski'],
		['  Jan  Maria   Kowalska ', 'Jan', 'Maria Kowalska'],
		['Cher', 'Cher', ''],
		['', '', '']
	])('splits %j', (name, firstName, lastName) => {
		expect(splitName(name)).toEqual({ firstName, lastName });
	});
});

describe('profileFormFromUser', () => {
	it('prefers the stored structured name', () => {
		const form = profileFormFromUser(
			user({ first_name: 'Anna Maria', last_name: 'Nowak', phone: '+48 600', patent_type: 'zeglarz_jachtowy', patent_number: 'ZJ-1' })
		);
		expect(form).toEqual({
			firstName: 'Anna Maria',
			lastName: 'Nowak',
			phone: '+48 600',
			patentType: 'zeglarz_jachtowy',
			patentNumber: 'ZJ-1'
		});
	});

	it('falls back to the display name for new accounts', () => {
		expect(profileFormFromUser(user({ name: 'Jan Kowalski' }))).toMatchObject({
			firstName: 'Jan',
			lastName: 'Kowalski',
			phone: '',
			patentType: ''
		});
	});

	it('uses the Firebase display name before the profile loads', () => {
		expect(profileFormFromUser(null, 'Ewa Klubowa')).toMatchObject({ firstName: 'Ewa', lastName: 'Klubowa' });
	});
});

describe('profileBody', () => {
	it('trims values and omits blank optionals', () => {
		expect(
			profileBody({ firstName: ' Jan ', lastName: ' Kowalski ', phone: '  ', patentType: '', patentNumber: 'X' })
		).toEqual({
			first_name: 'Jan',
			last_name: 'Kowalski',
			phone: undefined,
			patent_type: undefined,
			patent_number: undefined
		});
	});

	it('keeps the patent number only with a patent type', () => {
		expect(
			profileBody({ firstName: 'Jan', lastName: 'K', phone: '+48 1', patentType: 'kapitan_jachtowy', patentNumber: ' KPT-1 ' })
		).toMatchObject({ phone: '+48 1', patent_type: 'kapitan_jachtowy', patent_number: 'KPT-1' });
	});
});

describe('patentSummary', () => {
	it.each([
		[undefined, undefined, 'brak patentu'],
		['jachtowy_sternik_morski', 'JSM-1', 'Jachtowy Sternik Morski · JSM-1'],
		['kapitan_jachtowy', null, 'Kapitan Jachtowy']
	])('renders %s/%s', (type, number, want) => {
		expect(patentSummary(type, number)).toBe(want);
	});
});
