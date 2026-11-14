import { describe, it, expect, vi, afterEach } from 'vitest';
import { noteFailure, settle } from './errors';

afterEach(() => {
	vi.restoreAllMocks();
});

describe('settle', () => {
	it('resolves to the value and reports nothing when the request succeeds', async () => {
		const onError = vi.fn();
		const log = vi.spyOn(console, 'error').mockImplementation(() => {});

		await expect(settle(Promise.resolve([1, 2]), [], onError)).resolves.toEqual([1, 2]);
		expect(onError).not.toHaveBeenCalled();
		expect(log).not.toHaveBeenCalled();
	});

	it('resolves to the fallback, logs and reports the error when the request fails', async () => {
		const onError = vi.fn();
		const log = vi.spyOn(console, 'error').mockImplementation(() => {});
		const err = new Error('500');

		await expect(settle(Promise.reject(err), null, onError)).resolves.toBeNull();
		expect(onError).toHaveBeenCalledWith(err);
		expect(log).toHaveBeenCalledWith(expect.any(String), err);
	});

	it('keeps one failed section from rejecting Promise.all over the others', async () => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
		const errors: string[] = [];

		const [crew, ports] = await Promise.all([
			settle(Promise.reject(new Error('down')), [], noteFailure(errors, 'załoga')),
			settle(Promise.resolve(['Gdynia']), [], noteFailure(errors, 'porty'))
		]);

		expect(crew).toEqual([]);
		expect(ports).toEqual(['Gdynia']);
		expect(errors).toEqual(['załoga']);
	});
});

describe('noteFailure', () => {
	it('records each label once', () => {
		const errors: string[] = [];
		const crew = noteFailure(errors, 'załoga');

		crew();
		crew();
		noteFailure(errors, 'porty')();

		expect(errors).toEqual(['załoga', 'porty']);
	});
});
