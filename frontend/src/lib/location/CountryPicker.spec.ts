import { describe, it, expect } from 'vitest';
import { render, screen, fireEvent, within } from '@testing-library/svelte';
import { pickedCountries, testCountries } from '#lib/test-utils.js';
import CountryPicker from './CountryPicker.svelte';

function renderPicker(value: string[] = []) {
	render(CountryPicker, { props: { id: 'countries', value, countries: testCountries } });
	return screen.getByRole('combobox');
}

const listbox = () => screen.getByRole('listbox', { hidden: true });
const optionNames = () =>
	within(listbox())
		.queryAllByRole('option', { hidden: true })
		.map((o) => o.textContent?.trim());

describe('CountryPicker', () => {
	it('shows the picked codes by Polish name and removes one', async () => {
		renderPicker(['NO', 'DK']);
		expect(pickedCountries()).toEqual(['Norwegia', 'Dania']);

		await fireEvent.click(screen.getByRole('button', { name: 'Usuń kraj: Norwegia' }));

		expect(pickedCountries()).toEqual(['Dania']);
	});

	it('offers the Polish names that match the typed text, ignoring accents', async () => {
		const input = renderPicker();
		await fireEvent.input(input, { target: { value: 'wlo' } });

		expect(input).toHaveAttribute('aria-expanded', 'true');
		expect(optionNames()).toEqual(['Włochy']);
	});

	it('picks the first match on Enter without submitting the form, then clears the text', async () => {
		const input = renderPicker(['DK']);
		await fireEvent.input(input, { target: { value: 'nor' } });

		const notPrevented = await fireEvent.keyDown(input, { key: 'Enter' });

		expect(notPrevented).toBe(false);
		expect(pickedCountries()).toEqual(['Dania', 'Norwegia']);
		expect(input).toHaveValue('');
	});

	it('moves through the matches with the arrow keys', async () => {
		const input = renderPicker();
		await fireEvent.input(input, { target: { value: 'a' } });
		const first = optionNames()[0];

		await fireEvent.keyDown(input, { key: 'ArrowDown' });
		const second = optionNames()[1];
		const active = within(listbox()).getByRole('option', { selected: true, hidden: true });
		expect(active).toHaveTextContent(second ?? '');
		expect(input).toHaveAttribute('aria-activedescendant', active.id);

		await fireEvent.keyDown(input, { key: 'ArrowUp' });
		expect(
			within(listbox()).getByRole('option', { selected: true, hidden: true })
		).toHaveTextContent(first ?? '');

		await fireEvent.keyDown(input, { key: 'ArrowDown' });
		await fireEvent.keyDown(input, { key: 'Enter' });
		expect(pickedCountries()).toEqual([second]);
	});

	it('picks a clicked match and stops offering it', async () => {
		const input = renderPicker();
		await fireEvent.focus(input);
		await fireEvent.click(within(listbox()).getByRole('option', { name: 'Szwecja' }));

		expect(pickedCountries()).toEqual(['Szwecja']);
		expect(optionNames()).not.toContain('Szwecja');
	});

	it('closes the list on Escape', async () => {
		const input = renderPicker();
		await fireEvent.input(input, { target: { value: 'ma' } });
		expect(listbox()).toBeVisible();

		await fireEvent.keyDown(input, { key: 'Escape' });

		expect(input).toHaveAttribute('aria-expanded', 'false');
		expect(listbox()).not.toBeVisible();
	});

	it('removes the last country on Backspace in an empty box', async () => {
		const input = renderPicker(['NO', 'SE']);
		await fireEvent.keyDown(input, { key: 'Backspace' });

		expect(pickedCountries()).toEqual(['Norwegia']);
	});

	it('says when no country matches', async () => {
		const input = renderPicker();
		await fireEvent.input(input, { target: { value: 'Atlantyda' } });

		expect(screen.getByRole('status')).toHaveTextContent('Brak pasującego kraju');
		expect(input).toHaveAttribute('aria-expanded', 'false');
	});
});
