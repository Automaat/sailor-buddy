import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import YachtSelect from './YachtSelect.svelte';
import type { Yacht } from '#lib/api/types.js';

vi.mock('#lib/api/routes.js', () => ({ createYacht: vi.fn() }));

const selma: Yacht = { id: 4, name: 'Selma', created_at: '', updated_at: '' };

describe('YachtSelect', () => {
	it('creates the yacht once when Enter is pressed repeatedly', async () => {
		let resolve!: (y: Yacht) => void;
		const create = vi.fn(() => new Promise<Yacht>((r) => (resolve = r)));
		render(YachtSelect, { props: { value: 0, yachts: [], create } });

		await fireEvent.click(screen.getByRole('button', { name: '+ Nowy jacht spoza listy' }));
		const name = screen.getByLabelText('Nazwa jachtu *');
		await fireEvent.input(name, { target: { value: 'Selma' } });
		await fireEvent.keyDown(name, { key: 'Enter' });
		await fireEvent.keyDown(name, { key: 'Enter' });
		resolve(selma);

		await waitFor(() => expect(screen.getByRole('combobox')).toHaveValue('4'));
		expect(create).toHaveBeenCalledTimes(1);
	});
});
