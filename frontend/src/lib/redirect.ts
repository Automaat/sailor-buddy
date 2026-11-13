/**
 * safeRedirect accepts only same-origin absolute paths from a ?redirect=
 * param so it can't be used for open redirects. Browsers treat a leading
 * backslash like a slash, so `/\evil.com` is rejected as well.
 */
export function safeRedirect(target: string | null | undefined, fallback = '/'): string {
	if (!target || !target.startsWith('/') || target.startsWith('//') || target.startsWith('/\\')) {
		return fallback;
	}
	return target;
}
