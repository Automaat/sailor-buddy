import { safeRedirect } from '#lib/redirect.js';

export const ONBOARDING_PATH = '/onboarding';

type Location = Pick<URL, 'pathname' | 'search'> & {
	searchParams: Pick<URLSearchParams, 'get'>;
};

/**
 * onboardingRedirect decides where the onboarding gate sends a signed-in
 * user: to the form until they finish it, and back to the original page once
 * they have. Returns null when the current page may render.
 */
export function onboardingRedirect(onboarded: boolean, url: Location): string | null {
	const onForm = url.pathname === ONBOARDING_PATH;
	if (!onboarded) {
		if (onForm || url.pathname.startsWith('/login')) return null;
		return `${ONBOARDING_PATH}?redirect=${encodeURIComponent(url.pathname + url.search)}`;
	}
	return onForm ? safeRedirect(url.searchParams.get('redirect')) : null;
}
