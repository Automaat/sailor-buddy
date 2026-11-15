import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import { makeCruise } from '#lib/test-utils.js';
import EditionPicker, { groupOf } from './EditionPicker.svelte';

const series = [
	{ id: 2, name: 'NNE', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' }
];
const cruises = [
	makeCruise({ id: 13, name: 'NNE 2025', series_id: 2, edition: '2025' }),
	makeCruise({ id: 20, name: 'Rejs na Hel' })
];

describe('groupOf', () => {
	it.each([
		['no cruise', 0, 'standalone'],
		['an edition of a series', 13, '2'],
		['a cruise without a series', 20, 'none'],
		['a cruise missing from the list', 99, 'none']
	])('%s', (_, cruiseID, want) => {
		expect(groupOf(cruiseID, cruises)).toBe(want);
	});
});

describe('EditionPicker', () => {
	it('offers standalone, each series and "Bez serii", standalone hiding the edition step', () => {
		render(EditionPicker, { group: 'standalone', value: 0, cruises, series });

		const options = Array.from(
			(screen.getByLabelText('Seria') as HTMLSelectElement).options,
			(o) => o.text
		);
		expect(options).toEqual(['Samodzielny (bez edycji)', 'NNE', 'Bez serii']);
		expect(screen.queryByLabelText('Edycja *')).not.toBeInTheDocument();
	});

	it("lists only the chosen series' editions, labelled with the edition", () => {
		render(EditionPicker, { group: '2', value: 0, cruises, series });

		expect(screen.getByRole('option', { name: '2025 — NNE 2025' })).toBeInTheDocument();
		expect(screen.queryByRole('option', { name: 'Rejs na Hel' })).not.toBeInTheDocument();
		expect(screen.getByLabelText('Edycja *')).toBeRequired();
	});

	it('keeps a locked edition that failed to load selectable', () => {
		render(EditionPicker, { group: 'none', value: 99, cruises: [], series, locked: true });

		expect(screen.getByLabelText('Edycja *')).toHaveValue('99');
		expect(screen.getByRole('option', { name: 'Wydarzenie #99' })).toBeInTheDocument();
		expect(screen.getByLabelText('Seria')).toBeDisabled();
		expect(screen.getByLabelText('Edycja *')).toBeDisabled();
	});
});
