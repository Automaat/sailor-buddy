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

describe('YachtSelect when a create ends late', () => {
	it('adds the yacht to the bound list after the picker unmounts', async () => {
		const { create, resolvers } = pending();
		const props = $state({ value: 0, yachts: [wega], create });
		const { unmount } = render(YachtSelect, { props });

		await fillDraft('Selma');
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		unmount();
		resolvers[0](selma);

		await waitFor(() => expect(props.yachts).toEqual([wega, selma]));
		expect(props.value).toBe(0);
	});

	it('keeps the yacht the user picked while the create was pending', async () => {
		const { create, resolvers } = pending();
		const { submit, submitted, props } = renderInForm(create);
		props.yachts = [wega];

		await fillDraft('Selma');
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		await fireEvent.change(screen.getByRole('combobox'), { target: { value: '5' } });
		await fireEvent.click(submit);
		expect(submitted).toEqual([]);

		resolvers[0](selma);
		await waitFor(() => expect(submitted).toEqual([5]));
		expect(screen.getByRole('combobox')).toHaveValue('5');
		expect(screen.getByRole('option', { name: 'Selma' })).toBeInTheDocument();
		expect(screen.queryByLabelText('Nazwa jachtu *')).not.toBeInTheDocument();
	});

	it('keeps the earlier yacht when the user picks it again while the create is pending', async () => {
		const { create, resolvers } = pending();
		const other: Yacht = { id: 7, name: 'Other', created_at: '', updated_at: '' };
		const props = $state({ value: 5, yachts: [wega, other], create });
		render(YachtSelect, { props });

		await fillDraft('Selma');
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		await fireEvent.change(screen.getByRole('combobox'), { target: { value: '7' } });
		await fireEvent.change(screen.getByRole('combobox'), { target: { value: '5' } });
		resolvers[0](selma);

		await waitFor(() => expect(screen.getByRole('option', { name: 'Selma' })).toBeInTheDocument());
		expect(props.value).toBe(5);
		expect(screen.getByRole('combobox')).toHaveValue('5');
	});

	it('selects a listed yacht with the same type and name instead of creating it again', async () => {
		const create = vi.fn();
		const props = $state({
			value: 0,
			yachts: [wega, { ...selma, yacht_type: 'Hanse 415' }],
			create
		});
		render(YachtSelect, { props });

		await fillDraft(' selma ');
		await fireEvent.input(screen.getByLabelText('Typ jachtu'), { target: { value: 'HANSE 415' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));

		expect(create).not.toHaveBeenCalled();
		expect(props.value).toBe(4);
		expect(screen.getByRole('combobox')).toHaveValue('4');
		expect(screen.queryByLabelText('Nazwa jachtu *')).not.toBeInTheDocument();
	});

	it('creates a yacht whose name is listed under another type', async () => {
		const { create, resolvers } = pending();
		const props = $state({ value: 0, yachts: [{ ...selma, yacht_type: 'Hanse 415' }], create });
		render(YachtSelect, { props });

		await fillDraft('Selma');
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		resolvers[0]({ ...selma, id: 6 });

		await waitFor(() => expect(props.value).toBe(6));
		expect(create).toHaveBeenCalledTimes(1);
	});
});

describe('YachtSelect while a create is pending', () => {
	it('reuses the pending create when the yacht is added again after a cancel', async () => {
		const { create, resolvers } = pending();
		const props = $state({ value: 0, yachts: [] as Yacht[], create });
		render(YachtSelect, { props });

		await fillDraft('Selma');
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		await fireEvent.click(screen.getByRole('button', { name: 'Anuluj' }));
		await fillDraft(' SELMA ');
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		resolvers[0](selma);

		await waitFor(() => expect(props.value).toBe(4));
		expect(create).toHaveBeenCalledTimes(1);
		expect(props.yachts).toEqual([selma]);
		expect(screen.getByRole('combobox')).toHaveValue('4');
		expect(screen.queryByLabelText('Nazwa jachtu *')).not.toBeInTheDocument();
	});

	it('reuses the pending create when the yacht is added again after a remount', async () => {
		const { create, resolvers } = pending();
		const props = $state({ value: 0, yachts: [] as Yacht[], create });
		const { unmount } = render(YachtSelect, { props });

		await fillDraft('Selma');
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		unmount();

		const other = pending();
		const again = $state({ value: 0, yachts: [] as Yacht[], create: other.create });
		render(YachtSelect, { props: again });
		await fillDraft('Selma');
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		resolvers[0](selma);

		await waitFor(() => expect(again.value).toBe(4));
		expect(create).toHaveBeenCalledTimes(1);
		expect(other.create).not.toHaveBeenCalled();
		expect(again.yachts).toEqual([selma]);
		expect(screen.getByRole('combobox')).toHaveValue('4');
	});

	it('sends a new create after the pending one fails', async () => {
		const { create, resolvers, rejecters } = pending();
		const props = $state({ value: 0, yachts: [] as Yacht[], create });
		render(YachtSelect, { props });

		await fillDraft('Selma');
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		rejecters[0](new Error('Brak uprawnień'));
		expect(await screen.findByText('Brak uprawnień')).toBeInTheDocument();

		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj jacht' }));
		resolvers[1](selma);

		await waitFor(() => expect(props.value).toBe(4));
		expect(create).toHaveBeenCalledTimes(2);
	});
});
