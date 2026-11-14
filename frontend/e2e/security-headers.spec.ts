import { expect, test } from '@playwright/test';
import { createTestUser, uniqueEmail, TEST_PASSWORD } from './helpers';

test('production HTML blocks injected scripts and preserves asset caching', async ({ page }) => {
	const scripts: string[] = [];
	page.on('response', (response) => {
		if (response.url().includes('/_app/immutable/') && response.url().endsWith('.js'))
			scripts.push(response.url());
	});
	const response = await page.goto('/login');
	expect(response).not.toBeNull();
	const headers = response!.headers();
	expect(headers['content-security-policy']).toContain("frame-ancestors 'none'");
	expect(headers['content-security-policy']).toContain("'sha256-");
	expect(headers['content-security-policy']).not.toContain("script-src 'unsafe-inline'");
	expect(headers['cache-control']).toBe('private, no-store');
	expect(headers['referrer-policy']).toBe('no-referrer');
	expect(headers['x-content-type-options']).toBe('nosniff');
	expect(headers['x-frame-options']).toBe('DENY');
	await expect(page.getByRole('button', { name: 'Zaloguj', exact: true })).toBeVisible();
	await page.getByRole('button', { name: 'Zarejestruj', exact: true }).click();
	await expect(page.getByLabel('Imię', { exact: true })).toBeVisible();
	const blocked = await page.evaluate(async () => {
		const violation = new Promise<string>((resolve) =>
			document.addEventListener(
				'securitypolicyviolation',
				(event) => resolve(event.effectiveDirective),
				{ once: true }
			)
		);
		const script = document.createElement('script');
		script.textContent = 'document.documentElement.dataset.injected="executed"';
		document.body.append(script);
		return violation;
	});
	expect(blocked).toBe('script-src-elem');
	expect(await page.locator('html').getAttribute('data-injected')).toBeNull();
	expect(scripts.length).toBeGreaterThan(0);
	const asset = await page.request.get(scripts[0]);
	expect(asset.headers()['cache-control']).toContain('immutable');
	expect(asset.headers()['cache-control']).not.toContain('no-store');
});

test('signed-in API data and unauthenticated downloads cannot be cached', async ({ request }) => {
	const user = await createTestUser(uniqueEmail('cache-policy'), TEST_PASSWORD, 'Cache Policy');
	const api = process.env.API_BASE_URL || 'http://localhost:8080/api';
	const own = await request.get(`${api}/auth/me`, {
		headers: { Authorization: `Bearer ${user.idToken}` }
	});
	expect(own.status()).toBe(200);
	const download = await request.get(`${api}/voyages/1/opinions/1/download`);
	expect(download.status()).toBe(401);
	for (const response of [own, download]) {
		expect(response.headers()['cache-control']).toBe('private, no-store');
		expect(response.headers()['x-content-type-options']).toBe('nosniff');
		expect(response.headers()['referrer-policy']).toBe('no-referrer');
	}
});

test('the browser refuses to frame the app', async ({ page }) => {
	const violations: string[] = [];
	page.on('console', (message) => {
		if (message.text().includes('frame-ancestors')) violations.push(message.text());
	});
	await page.goto('/login');
	await page.evaluate(() => {
		const frame = document.createElement('iframe');
		frame.src = '/login';
		document.body.append(frame);
	});
	await expect.poll(() => violations.length).toBeGreaterThan(0);
});

test('enrollment links do not leak through map image referrers', async ({ page }) => {
	let referrer: string | undefined;
	await page.route('https://a.tile.openstreetmap.org/0/0/0.png', async (route) => {
		referrer = route.request().headers().referer;
		await route.fulfill({
			contentType: 'image/png',
			body: Buffer.from(
				'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a/3sAAAAASUVORK5CYII=',
				'base64'
			)
		});
	});
	await page.goto('/enroll/private-referrer-probe');
	const loaded = await page.evaluate(
		() =>
			new Promise<boolean>((resolve) => {
				const image = new Image();
				image.onload = () => resolve(image.naturalWidth > 0);
				image.onerror = () => resolve(false);
				image.src = 'https://a.tile.openstreetmap.org/0/0/0.png';
				document.body.append(image);
			})
	);
	expect(loaded).toBe(true);
	expect(referrer).toBeUndefined();
});
