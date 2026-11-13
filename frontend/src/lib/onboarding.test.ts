import { describe, it, expect } from 'vitest';
import { onboardingRedirect } from './onboarding.js';

const url = (path: string) => new URL(path, 'http://localhost');

describe('onboardingRedirect', () => {
	it.each([
		[false, '/', '/onboarding?redirect=%2F'],
		[false, '/enroll/tok?x=1', '/onboarding?redirect=%2Fenroll%2Ftok%3Fx%3D1'],
		[false, '/onboarding?redirect=%2Fcruises', null],
		[false, '/login', null],
		[true, '/cruises', null],
		[true, '/onboarding?redirect=%2Fenroll%2Ftok', '/enroll/tok'],
		[true, '/onboarding', '/'],
		[true, '/onboarding?redirect=%2F%2Fevil.com', '/']
	])('onboarded=%s at %s -> %s', (onboarded, path, want) => {
		expect(onboardingRedirect(onboarded, url(path))).toBe(want);
	});
});
