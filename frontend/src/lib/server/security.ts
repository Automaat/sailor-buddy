type RuntimeSettings = {
	apiURL: string;
	authDomain: string;
	emulatorURL?: string;
	dev: boolean;
};

export function secureResponse(response: Response, url: URL, settings: RuntimeSettings): Response {
	const headers = new Headers(response.headers);
	headers.set('x-content-type-options', 'nosniff');
	headers.set('x-frame-options', 'DENY');
	headers.set('referrer-policy', 'no-referrer');
	headers.set('cache-control', 'private, no-store');
	const policy = headers.get('content-security-policy');
	if (policy) {
		const connect = new Set([
			"'self'",
			'https://identitytoolkit.googleapis.com',
			'https://securetoken.googleapis.com',
			'https://www.googleapis.com'
		]);
		const frames = new Set(["'self'", origin(`https://${settings.authDomain}`)]);
		connect.add(origin(new URL(settings.apiURL || '/api', url).href));
		if (settings.emulatorURL) {
			const emulator = origin(settings.emulatorURL);
			connect.add(emulator);
			frames.add(emulator);
		}
		if (settings.dev) connect.add(`${url.protocol === 'https:' ? 'wss:' : 'ws:'}//${url.host}`);
		headers.set(
			'content-security-policy',
			`${policy}; default-src 'none'; connect-src ${[...connect].join(' ')}; frame-src ${[...frames].join(' ')}`
		);
	} else if (headers.get('content-type')?.includes('text/html')) {
		headers.set(
			'content-security-policy',
			"default-src 'none'; frame-ancestors 'none'; base-uri 'none'"
		);
	}
	return new Response(response.body, {
		status: response.status,
		statusText: response.statusText,
		headers
	});
}

function origin(value: string): string {
	const parsed = new URL(value);
	if (
		!['http:', 'https:'].includes(parsed.protocol) ||
		parsed.username ||
		parsed.password ||
		/[\s;,'"<>*]/.test(parsed.origin)
	)
		throw new Error('Invalid browser service origin');
	return parsed.origin;
}
