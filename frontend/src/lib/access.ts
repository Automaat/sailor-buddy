import { ONBOARDING_PATH } from '#lib/onboarding.js';
import type { User } from '#lib/api/types.js';

/** The page that explains why club data is not available yet. */
export const ACCESS_PATH = '/pending';

/** The public enrollment preview shared via invite link. Matched on the
 * exact segment so an unrelated route like /enrollments stays private. */
export function isEnrollRoute(pathname: string): boolean {
	return pathname === '/enroll' || pathname.startsWith('/enroll/');
}

/** Routes a logged-out visitor may reach: the login screen and the
 * enrollment preview. */
export function isPublicRoute(pathname: string): boolean {
	return pathname.startsWith('/login') || isEnrollRoute(pathname);
}

/** hasClubAccess mirrors the API's admission rule: a verified email and an
 * account that is not suspended. */
export function hasClubAccess(user: Pick<User, 'email_verified' | 'suspended'>): boolean {
	return user.email_verified && !user.suspended;
}

/**
 * isAdminOrCaptainGroup mirrors the API's rule for the captains' working
 * data, such as hidden drafts and the yacht list (Jachty): admins and the
 * captain group (users.is_captain). It is false until /auth/me answers.
 */
export function isAdminOrCaptainGroup(auth: {
	isAdmin: boolean;
	user: Pick<User, 'is_captain'> | null | undefined;
}): boolean {
	return auth.isAdmin || !!auth.user?.is_captain;
}

/**
 * accessRedirect decides where the access gate sends an onboarded user:
 * without club access, to ACCESS_PATH unless the page works without it.
 * Returns null when the current page may render.
 */
export function accessRedirect(
	user: Pick<User, 'email_verified' | 'suspended'>,
	pathname: string
): string | null {
	if (hasClubAccess(user) || isPublicRoute(pathname)) return null;
	if ([ACCESS_PATH, '/profile', ONBOARDING_PATH].includes(pathname)) return null;
	return ACCESS_PATH;
}
