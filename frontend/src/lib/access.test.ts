import { describe, expect, it } from 'vitest';
import { ACCESS_PATH, accessRedirect, hasClubAccess } from '#lib/access.js';

const verified = { email_verified: true, suspended: false };
const unverified = { email_verified: false, suspended: false };
const suspended = { email_verified: true, suspended: true };

describe('hasClubAccess', () => {
	it.each([
		['verified email', verified, true],
		['unverified email', unverified, false],
		['suspended account', suspended, false],
		['suspended and unverified', { email_verified: false, suspended: true }, false]
	])('%s', (_, user, want) => {
		expect(hasClubAccess(user)).toBe(want);
	});
});

describe('accessRedirect', () => {
	it.each([
		['verified user reads club pages', verified, '/cruises', null],
		['verified user on the access page', verified, ACCESS_PATH, null],
		['unverified user on a club page', unverified, '/cruises', ACCESS_PATH],
		['unverified user on the dashboard', unverified, '/', ACCESS_PATH],
		['suspended user on a club page', suspended, '/crew', ACCESS_PATH],
		['unverified user on the access page', unverified, ACCESS_PATH, null],
		['unverified user edits the profile', unverified, '/profile', null],
		['unverified user finishes onboarding', unverified, '/onboarding', null],
		['unverified user opens a share link', unverified, '/enroll/abc', null],
		['unverified user on the login page', unverified, '/login', null],
		[
			'unverified user on a route that only starts like enroll',
			unverified,
			'/enrollments',
			ACCESS_PATH
		]
	])('%s', (_, user, pathname, want) => {
		expect(accessRedirect(user, pathname)).toBe(want);
	});
});
