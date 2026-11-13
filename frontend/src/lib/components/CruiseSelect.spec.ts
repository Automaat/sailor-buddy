import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
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
});
