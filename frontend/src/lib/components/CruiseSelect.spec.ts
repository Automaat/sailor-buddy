import { describe, it, expect, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import { makeCruise } from '#lib/test-utils.js';
import CruiseSelect from './CruiseSelect.svelte';

describe('CruiseSelect', () => {
	it('lists the cruises with a disabled placeholder', () => {
		render(CruiseSelect, { value: 0, cruises: [makeCruise({ id: 1, name: 'NNE' })] });
		expect(screen.getByRole('option', { name: '-- Wybierz wydarzenie --' })).toBeDisabled();
		expect(screen.getByRole('option', { name: 'NNE' })).toBeInTheDocument();
		expect(screen.getByRole('combobox')).toBeRequired();
	});

	it('adds a fallback option for a linked cruise missing from the list', () => {
		render(CruiseSelect, { value: 9, cruises: [] });
		expect(screen.getByRole('combobox')).toHaveValue('9');
		expect(screen.getByRole('option', { name: 'Wydarzenie #9' })).toBeInTheDocument();
	});

	it('adds no fallback when the linked cruise is listed', () => {
		render(CruiseSelect, { value: 1, cruises: [makeCruise({ id: 1, name: 'NNE' })] });
		expect(screen.queryByRole('option', { name: 'Wydarzenie #1' })).not.toBeInTheDocument();
	});

	it('reports the newly selected cruise id to onchange', async () => {
		const onchange = vi.fn();
		render(CruiseSelect, {
			value: 1,
			cruises: [makeCruise({ id: 1, name: 'NNE' }), makeCruise({ id: 2, name: 'WRM' })],
			onchange
		});
		await fireEvent.change(screen.getByRole('combobox'), { target: { value: '2' } });
		expect(onchange).toHaveBeenCalledWith(2);
	});
});
