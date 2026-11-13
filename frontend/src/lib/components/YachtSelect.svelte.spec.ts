import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import YachtSelect from './YachtSelect.svelte';
import type { Yacht } from '#lib/api/types.js';

vi.mock('#lib/api/routes.js', () => ({ createYacht: vi.fn() }));

const selma: Yacht = { id: 4, name: 'Selma', created_at: '', updated_at: '' };
const wega: Yacht = { id: 5, name: 'Wega', created_at: '', updated_at: '' };

function pending() {
	const resolvers: ((y: Yacht) => void)[] = [];
	const rejecters: ((e: Error) => void)[] = [];
	const create = vi.fn(
		() =>
			new Promise<Yacht>((res, rej) => {
				resolvers.push(res);
				rejecters.push(rej);
			})
	);
	return { create, resolvers, rejecters };
}

// Renders the picker inside a form whose submit handler reads the bound value
// the way the voyage and leg forms do.
function renderInForm(create: (body: unknown) => Promise<Yacht>) {
	const form = document.createElement('form');
	const submit = document.createElement('button');
	submit.type = 'submit';
	submit.textContent = 'Zapisz';
	document.body.appendChild(form);
	const props = $state({ value: 0, yachts: [] as Yacht[], create });
	const submitted: number[] = [];
	form.addEventListener('submit', (e) => {
		e.preventDefault();
		submitted.push(props.value);
	});
	render(YachtSelect, { target: form, props });
	form.appendChild(submit);
	return { form, submit, submitted, props };
}

async function fillDraft(name: string) {
	await fireEvent.click(screen.getByRole('button', { name: '+ Nowy jacht spoza listy' }));
	await fireEvent.input(screen.getByLabelText('Nazwa jachtu *'), { target: { value: name } });
}

afterEach(() => {
	document.body.innerHTML = '';
});

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

	it('holds the parent form submit until the new yacht is selected', async () => {
		const { create, resolvers } = pending();
		const { submit, submitted } = renderInForm(create);

		await fillDraft('Selma');
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		await fireEvent.click(submit);
		expect(submitted).toEqual([]);

		resolvers[0](selma);
		await waitFor(() => expect(submitted).toEqual([4]));
		expect(screen.getByRole('combobox')).toHaveValue('4');
	});

	it('drops the held submit when the create fails', async () => {
		const { create, rejecters } = pending();
		const { submit, submitted } = renderInForm(create);

		await fillDraft('Selma');
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		await fireEvent.click(submit);
		rejecters[0](new Error('Brak uprawnień'));

		expect(await screen.findByText('Brak uprawnień')).toBeInTheDocument();
		expect(submitted).toEqual([]);
		await fireEvent.click(submit);
		expect(submitted).toEqual([0]);
	});

	it('ignores a cancelled create that resolves after a new draft is opened', async () => {
		const { create, resolvers } = pending();
		const { submit, submitted } = renderInForm(create);

		await fillDraft('Selma');
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Anuluj' }));
		await fillDraft('Wega');

		resolvers[0](selma);
		await waitFor(() => expect(screen.getByRole('option', { name: 'Selma' })).toBeInTheDocument());
		expect(screen.getByRole('combobox')).toHaveValue('0');
		expect(screen.getByLabelText('Nazwa jachtu *')).toHaveValue('Wega');
		expect(screen.getByRole('button', { name: 'Dodaj jacht' })).toBeEnabled();

		await fireEvent.click(submit);
		expect(submitted).toEqual([0]);

		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		resolvers[1](wega);
		await waitFor(() => expect(screen.getByRole('combobox')).toHaveValue('5'));
		expect(screen.queryByLabelText('Nazwa jachtu *')).not.toBeInTheDocument();
	});
});
