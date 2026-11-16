import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import PortField from './PortField.svelte';

const geocodeMock = vi.fn();
const suggestMock = vi.fn();
vi.mock('#lib/api/routes.js', () => ({
	geocode: (q: string, bias?: unknown) => geocodeMock(q, bias),
	suggestPorts: (q: string) => suggestMock(q)
}));

const LABEL = 'Port wyjścia';
const field = () => screen.getByLabelText(LABEL) as HTMLInputElement;
const type = (value: string) => fireEvent.input(field(), { target: { value } });
const searchButton = () => screen.getByRole('button', { name: `Szukaj miejsca: ${LABEL}` });

function renderField(props: Record<string, unknown> = {}) {
	return render(PortField, {
		props: { id: 'start_port', label: LABEL, value: { name: '' }, ...props }
	});
}

describe('PortField', () => {
	beforeEach(() => {
		geocodeMock.mockReset();
		suggestMock.mockReset();
		suggestMock.mockResolvedValue([]);
		geocodeMock.mockResolvedValue([]);
	});

	it('labels the input and its search button', () => {
		renderField({ value: { name: 'Hel' } });
		expect(field()).toHaveValue('Hel');
		expect(searchButton()).toHaveTextContent('Szukaj');
	});

	it('suggests club ports after typing pauses, without looking places up', async () => {
		suggestMock.mockResolvedValue([{ name: 'Hel', latitude: 54.6, longitude: 18.8, uses: 2 }]);
		vi.useFakeTimers();
		try {
			renderField();
			await type('h');
			await type('he');
			await vi.advanceTimersByTimeAsync(249);
			expect(suggestMock).not.toHaveBeenCalled();
			await vi.advanceTimersByTimeAsync(1);
			expect(suggestMock).toHaveBeenCalledExactlyOnceWith('he');
			await vi.advanceTimersByTimeAsync(5000);
			expect(geocodeMock).not.toHaveBeenCalled();
		} finally {
			vi.useRealTimers();
		}
		expect(await screen.findByRole('button', { name: /^Hel/ })).toHaveTextContent('2×');
	});

	it('fills the name and coordinates from a picked suggestion', async () => {
		suggestMock.mockResolvedValue([{ name: 'Hel', latitude: 54.6, longitude: 18.8, uses: 2 }]);
		const onpick = vi.fn();
		renderField({ onpick });
		await type('he');
		await fireEvent.click(await screen.findByRole('button', { name: /^Hel/ }));

		expect(onpick).toHaveBeenCalledWith({ name: 'Hel', latitude: 54.6, longitude: 18.8 });
		expect(field()).toHaveValue('Hel');
		expect(screen.getByText('54.60000, 18.80000')).toBeInTheDocument();
		expect(screen.queryByRole('button', { name: /^Hel/ })).toBeNull();
		expect(geocodeMock).not.toHaveBeenCalled();
	});

	it('fills only the name from a suggestion without a location', async () => {
		suggestMock.mockResolvedValue([{ name: 'Zatoka Pucka', uses: 1 }]);
		const onpick = vi.fn();
		renderField({ onpick });
		await type('zat');
		const sug = await screen.findByRole('button', { name: /Zatoka Pucka/ });
		expect(sug).toHaveTextContent('bez mapy');
		await fireEvent.click(sug);
		expect(onpick).toHaveBeenCalledWith({ name: 'Zatoka Pucka' });
		expect(screen.queryByText(/Na mapie/)).toBeNull();
	});

	it('clears the coordinates once the name changes and restores them for the same name', async () => {
		renderField({ value: { name: 'Hel', latitude: 54.6, longitude: 18.8 } });
		expect(screen.getByText('54.60000, 18.80000')).toBeInTheDocument();

		await type('Hela');
		expect(screen.queryByText(/Na mapie/)).toBeNull();
		await type(' Hel ');
		expect(screen.getByText('54.60000, 18.80000')).toBeInTheDocument();
		await type('Jastarnia');
		expect(screen.queryByText(/Na mapie/)).toBeNull();
	});

	it('forgets the coordinates of a value the parent replaced', async () => {
		const { rerender } = renderField({ value: { name: 'Hel', latitude: 54.6, longitude: 18.8 } });
		await type('Hela');
		await rerender({ value: { name: 'Gdynia' } });
		await type('Hel');
		expect(screen.queryByText(/Na mapie/)).toBeNull();
	});

	it('looks the place up on Enter without submitting the form and picks a result', async () => {
		geocodeMock.mockResolvedValue([
			{
				name: 'Visby',
				label: 'Visby, Gotland, Sweden',
				latitude: 57.64,
				longitude: 18.29,
				source: 'nominatim'
			}
		]);
		const onSubmit = vi.fn((e: Event) => e.preventDefault());
		const form = document.createElement('form');
		form.addEventListener('submit', onSubmit);
		document.body.appendChild(form);
		const onpick = vi.fn();
		try {
			render(PortField, {
				target: form,
				props: {
					id: 'end_port',
					label: LABEL,
					value: { name: '' },
					onpick,
					bias: () => ({ lat: 54, lon: 15 })
				}
			});
			await type('Visby');
			const notCancelled = await fireEvent.keyDown(field(), { key: 'Enter' });
			expect(notCancelled).toBe(false);
			expect(onSubmit).not.toHaveBeenCalled();
			expect(geocodeMock).toHaveBeenCalledExactlyOnceWith('Visby', { lat: 54, lon: 15 });

			expect(await screen.findByText('Visby, Gotland, Sweden')).toBeInTheDocument();
			expect(screen.getByRole('link', { name: 'współtwórcy OpenStreetMap' })).toBeInTheDocument();
			await fireEvent.click(screen.getByRole('button', { name: /^Visby/ }));
			expect(onpick).toHaveBeenCalledWith({ name: 'Visby', latitude: 57.64, longitude: 18.29 });
			expect(screen.getByText('57.64000, 18.29000')).toBeInTheDocument();
		} finally {
			form.remove();
		}
	});

	it('looks the place up on the Szukaj button', async () => {
		renderField();
		await type('Visby');
		await fireEvent.click(searchButton());
		await waitFor(() => expect(geocodeMock).toHaveBeenCalledExactlyOnceWith('Visby', undefined));
		expect(await screen.findByText('Brak wyników')).toBeInTheDocument();
	});

	it('keeps the results of a Szukaj click once the button disables itself', async () => {
		let resolve: (v: unknown[]) => void = () => {};
		geocodeMock.mockReturnValue(new Promise((r) => (resolve = r)));
		renderField();
		await type('Hel');
		searchButton().focus();
		await fireEvent.click(searchButton());
		// A browser moves focus off a button it disables, which fires focusout
		// on the field; focus must already be on the input by then.
		expect(document.activeElement).toBe(field());
		await fireEvent.focusOut(field(), { relatedTarget: searchButton() });
		resolve([
			{ name: 'Hel', label: 'Hel, Poland', latitude: 54.6, longitude: 18.8, source: 'nominatim' }
		]);
		expect(await screen.findByText('Hel, Poland')).toBeInTheDocument();
	});

	it('does not look up a name shorter than two characters', async () => {
		renderField();
		await type('V');
		expect(searchButton()).toBeDisabled();
		await fireEvent.keyDown(field(), { key: 'Enter' });
		expect(geocodeMock).not.toHaveBeenCalled();
	});

	it('moves through the list with the arrow keys and closes it on Escape', async () => {
		suggestMock.mockResolvedValue([
			{ name: 'Hel', latitude: 54.6, longitude: 18.8, uses: 2 },
			{ name: 'Helsinki', latitude: 60.17, longitude: 24.94, uses: 1 }
		]);
		renderField();
		await type('hel');
		const first = await screen.findByRole('button', { name: /^Hel\b/ });
		const second = screen.getByRole('button', { name: /^Helsinki/ });

		await fireEvent.keyDown(field(), { key: 'ArrowDown' });
		expect(first).toHaveFocus();
		await fireEvent.keyDown(first, { key: 'ArrowDown' });
		expect(second).toHaveFocus();
		await fireEvent.keyDown(second, { key: 'ArrowDown' });
		expect(second).toHaveFocus();
		await fireEvent.keyDown(second, { key: 'ArrowUp' });
		await fireEvent.keyDown(first, { key: 'ArrowUp' });
		expect(field()).toHaveFocus();

		await fireEvent.keyDown(field(), { key: 'ArrowDown' });
		await fireEvent.keyDown(first, { key: 'Escape' });
		expect(field()).toHaveFocus();
		expect(screen.queryByText('Wcześniej odwiedzone')).toBeNull();
	});

	it('closes the list when focus leaves the field', async () => {
		suggestMock.mockResolvedValue([{ name: 'Hel', uses: 1 }]);
		renderField();
		await type('hel');
		await screen.findByText('Wcześniej odwiedzone');
		await fireEvent.focusOut(field(), { relatedTarget: document.body });
		expect(screen.queryByText('Wcześniej odwiedzone')).toBeNull();
	});

	it('keeps the list open while focus moves into it', async () => {
		suggestMock.mockResolvedValue([{ name: 'Hel', uses: 1 }]);
		renderField();
		await type('hel');
		const option = await screen.findByRole('button', { name: /^Hel/ });
		await fireEvent.focusOut(field(), { relatedTarget: option });
		expect(screen.getByText('Wcześniej odwiedzone')).toBeInTheDocument();
	});

	it('gives two fields on one page their own lists and buttons', async () => {
		suggestMock.mockResolvedValue([{ name: 'Hel', uses: 1 }]);
		render(PortField, { props: { id: 'start_port', label: 'Port wyjścia', value: { name: '' } } });
		render(PortField, { props: { id: 'end_port', label: 'Port docelowy', value: { name: '' } } });
		expect(
			screen.getByRole('button', { name: 'Szukaj miejsca: Port wyjścia' })
		).toBeInTheDocument();
		expect(
			screen.getByRole('button', { name: 'Szukaj miejsca: Port docelowy' })
		).toBeInTheDocument();
		await fireEvent.input(screen.getByLabelText('Port docelowy'), { target: { value: 'he' } });
		expect(await screen.findByRole('group', { name: 'Wcześniej odwiedzone' })).toBeInTheDocument();
		expect(screen.getAllByRole('group')).toHaveLength(1);
	});
});
