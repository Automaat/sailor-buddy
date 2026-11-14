import { describe, expect, it } from 'vitest';
import { secureResponse } from './security';

const settings = {
	apiURL: 'https://api.example.com/api',
	authDomain: 'login.example.com',
	dev: false
};
const url = new URL('https://club.example.com/enroll/private-token');
const policy = "script-src 'self' 'sha256-framework-hash'; frame-ancestors 'none'";

describe('browser response security', () => {
	it('preserves framework script authorization and limits runtime service origins', async () => {
		const response = secureResponse(
			new Response('<html>app</html>', {
				headers: {
					'content-type': 'text/html',
					'content-security-policy': policy,
					'cache-control': 'public, max-age=600'
				}
			}),
			url,
			settings
		);
		expect(response.headers.get('content-security-policy')).toBe(
			`${policy}; default-src 'none'; connect-src 'self' https://identitytoolkit.googleapis.com https://securetoken.googleapis.com https://www.googleapis.com https://api.example.com; frame-src 'self' https://login.example.com`
		);
		expect(response.headers.get('cache-control')).toBe('private, no-store');
		expect(response.headers.get('referrer-policy')).toBe('no-referrer');
		expect(response.headers.get('x-frame-options')).toBe('DENY');
		expect(response.headers.get('x-content-type-options')).toBe('nosniff');
		expect(await response.text()).toBe('<html>app</html>');
	});
	it('allows only the selected emulator and development socket', () => {
		const response = secureResponse(
			new Response(null, { headers: { 'content-security-policy': policy } }),
			new URL('http://127.0.0.1:5173'),
			{ ...settings, emulatorURL: 'http://127.0.0.1:9099', dev: true }
		);
		const header = response.headers.get('content-security-policy');
		expect(header).toContain('http://127.0.0.1:9099');
		expect(header).toContain('ws://127.0.0.1:5173');
		expect(header).not.toContain('unsafe-inline');
	});
	it('protects redirects and error HTML even without framework CSP', () => {
		for (const response of [
			Response.redirect('https://club.example.com/login', 303),
			new Response('failure', { status: 500, headers: { 'content-type': 'text/html' } })
		]) {
			const secured = secureResponse(response, url, settings);
			expect(secured.status).toBe(response.status);
			expect(secured.headers.get('cache-control')).toBe('private, no-store');
			expect(secured.headers.get('x-frame-options')).toBe('DENY');
			if (response.status === 500)
				expect(secured.headers.get('content-security-policy')).toContain("default-src 'none'");
		}
	});
	it.each([
		'https://user:pass@api.example.com/api',
		'data:text/plain,x',
		'https://*.example.com/api',
		'https://foo;script-src/api'
	])('rejects invalid configured origins: %s', (apiURL) => {
		expect(() =>
			secureResponse(new Response(null, { headers: { 'content-security-policy': policy } }), url, {
				...settings,
				apiURL
			})
		).toThrow();
	});
});
