import { type Page } from '@playwright/test';

const FIREBASE_EMULATOR = process.env.FIREBASE_AUTH_EMULATOR_URL || 'http://localhost:9099';
const FIREBASE_PROJECT = process.env.FIREBASE_PROJECT_ID || 'sailor-buddy-dev';
const API_BASE = process.env.API_BASE_URL || 'http://localhost:5173/api';

interface FirebaseSignUpResponse {
	idToken: string;
	localId: string;
	email: string;
	refreshToken: string;
}

export async function createTestUser(
	email: string,
	password: string,
	displayName: string
): Promise<FirebaseSignUpResponse> {
	const res = await fetch(
		`${FIREBASE_EMULATOR}/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-api-key`,
		{
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email, password, displayName, returnSecureToken: true })
		}
	);
	if (!res.ok) throw new Error(`Failed to create user: ${await res.text()}`);
	return res.json();
}

export async function signInTestUser(
	email: string,
	password: string
): Promise<FirebaseSignUpResponse> {
	const res = await fetch(
		`${FIREBASE_EMULATOR}/identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=fake-api-key`,
		{
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ email, password, returnSecureToken: true })
		}
	);
	if (!res.ok) throw new Error(`Failed to sign in: ${await res.text()}`);
	return res.json();
}

export async function clearFirebaseUsers(): Promise<void> {
	await fetch(
		`${FIREBASE_EMULATOR}/emulator/v1/projects/${FIREBASE_PROJECT}/accounts`,
		{ method: 'DELETE' }
	);
}

export async function apiRequest(
	token: string,
	method: string,
	path: string,
	body?: unknown
): Promise<unknown> {
	const res = await fetch(`${API_BASE}${path}`, {
		method,
		headers: {
			'Content-Type': 'application/json',
			Authorization: `Bearer ${token}`
		},
		body: body ? JSON.stringify(body) : undefined
	});
	if (res.status === 204) return undefined;
	return res.json();
}

export async function loginViaUI(page: Page, email: string, password: string): Promise<void> {
	await page.goto('/login');
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Hasło').fill(password);
	await page.getByRole('button', { name: 'Zaloguj' }).click();
	await page.waitForURL('/', { timeout: 10_000 });
}

export async function registerViaUI(
	page: Page,
	name: string,
	email: string,
	password: string
): Promise<void> {
	await page.goto('/login');
	await page.getByRole('button', { name: 'Zarejestruj' }).first().click();
	await page.getByLabel('Imię').fill(name);
	await page.getByLabel('Email').fill(email);
	await page.getByLabel('Hasło').fill(password);
	await page.getByRole('button', { name: 'Zarejestruj' }).first().click();
	await page.waitForURL('/', { timeout: 10_000 });
}

let userCounter = 0;

export function uniqueEmail(prefix = 'e2e'): string {
	return `${prefix}-${Date.now()}-${++userCounter}@test.local`;
}
