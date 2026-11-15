import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import { ApiError } from '#lib/api/client.js';
import PortPicker from './PortPicker.svelte';

const geocodeMock = vi.fn();
const suggestMock = vi.fn();
vi.mock('#lib/api/routes.js', () => ({
	geocode: (q: string, bias?: unknown) => geocodeMock(q, bias),
	suggestPorts: (q: string) => suggestMock(q)
}));

describe('PortPicker', () => {
	beforeEach(() => {
		geocodeMock.mockReset();
		suggestMock.mockReset();
		suggestMock.mockResolvedValue([]);
	});

	it('renders existing ports numbered in order', () => {
		render(PortPicker, {
			props: {
				ports: [
					{ name: 'Split', latitude: 43.5, longitude: 16.4 },
					{ name: 'Hvar', latitude: 43.1, longitude: 16.4 }
				]
			}
		});
		expect(screen.getByText('Split')).toBeInTheDocument();
		expect(screen.getByText('Hvar')).toBeInTheDocument();
	});

	const typeQuery = (value: string) =>
		fireEvent.input(screen.getByLabelText('Odwiedzone porty'), { target: { value } });

	it('does not search while typing', async () => {
		geocodeMock.mockResolvedValue([]);
		vi.useFakeTimers();
		try {
			render(PortPicker, { props: { ports: [], onAdd: vi.fn() } });
			await typeQuery('Spl');
			await typeQuery('Split');
			await vi.advanceTimersByTimeAsync(5000);
			expect(geocodeMock).not.toHaveBeenCalled();
		} finally {
			vi.useRealTimers();
		}
	});

	it('searches on the Szukaj button and adds the picked result', async () => {
		geocodeMock.mockResolvedValue([{ name: 'Split', latitude: 43.5, longitude: 16.4 }]);
		const onAdd = vi.fn();
		render(PortPicker, { props: { ports: [], onAdd } });

		await typeQuery('Split');
		await fireEvent.click(screen.getByRole('button', { name: 'Szukaj' }));
		const result = await screen.findByRole('button', { name: 'Split' });
		await fireEvent.click(result);

		expect(geocodeMock).toHaveBeenCalledTimes(1);
		expect(geocodeMock.mock.calls[0][0]).toBe('Split');
		expect(onAdd).toHaveBeenCalledWith({ name: 'Split', latitude: 43.5, longitude: 16.4 });
	});

	it('searches on Enter without submitting an enclosing form', async () => {
		geocodeMock.mockResolvedValue([{ name: 'Hvar', latitude: 43.1, longitude: 16.4 }]);
		const onSubmit = vi.fn((e: Event) => e.preventDefault());
		const form = document.createElement('form');
		form.addEventListener('submit', onSubmit);
		document.body.appendChild(form);
		try {
			render(PortPicker, { target: form, props: { ports: [] } });
			await typeQuery('Hvar');
			const notCancelled = await fireEvent.keyDown(screen.getByLabelText('Odwiedzone porty'), {
				key: 'Enter'
			});

			expect(await screen.findByRole('button', { name: 'Hvar' })).toBeInTheDocument();
			expect(geocodeMock.mock.calls[0][0]).toBe('Hvar');
			expect(notCancelled).toBe(false);
			expect(onSubmit).not.toHaveBeenCalled();
		} finally {
			form.remove();
		}
	});

	it('suggests club ports while typing, once typing pauses', async () => {
		suggestMock.mockResolvedValue([
			{ name: 'Świnoujście', latitude: 53.91, longitude: 14.25, uses: 3 }
		]);
		vi.useFakeTimers();
		try {
			render(PortPicker, { props: { ports: [], onAdd: vi.fn() } });
			await typeQuery('sw');
			await typeQuery('swi');
			await vi.advanceTimersByTimeAsync(100);
			await typeQuery('swin');
			await vi.advanceTimersByTimeAsync(249);
			expect(suggestMock).not.toHaveBeenCalled();
			await vi.advanceTimersByTimeAsync(1);
			expect(suggestMock).toHaveBeenCalledTimes(1);
			expect(suggestMock).toHaveBeenCalledWith('swin');
			expect(geocodeMock).not.toHaveBeenCalled();
		} finally {
			vi.useRealTimers();
		}
		expect(await screen.findByText('Wcześniej odwiedzone')).toBeInTheDocument();
		expect(screen.getByRole('button', { name: /Świnoujście/ })).toHaveTextContent('3×');
	});

	it('does not ask for suggestions below two characters', async () => {
		vi.useFakeTimers();
		try {
			render(PortPicker, { props: { ports: [] } });
			await typeQuery('s');
			await vi.advanceTimersByTimeAsync(1000);
			expect(suggestMock).not.toHaveBeenCalled();
		} finally {
			vi.useRealTimers();
		}
	});

	it('adds a picked suggestion with its coordinates', async () => {
		suggestMock.mockResolvedValue([{ name: 'Hel', latitude: 54.6, longitude: 18.8, uses: 2 }]);
		const onAdd = vi.fn();
		render(PortPicker, { props: { ports: [], onAdd } });
		await typeQuery('he');
		await fireEvent.click(await screen.findByRole('button', { name: /^Hel/ }));
		expect(onAdd).toHaveBeenCalledWith({ name: 'Hel', latitude: 54.6, longitude: 18.8 });
		expect(geocodeMock).not.toHaveBeenCalled();
	});

	it('adds a name-only suggestion without coordinates', async () => {
		suggestMock.mockResolvedValue([{ name: 'Zatoka Pucka', uses: 1 }]);
		const onAdd = vi.fn();
		render(PortPicker, { props: { ports: [], onAdd } });
		await typeQuery('zat');
		const sug = await screen.findByRole('button', { name: /Zatoka Pucka/ });
		expect(sug).toHaveTextContent('bez mapy');
		await fireEvent.click(sug);
		expect(onAdd).toHaveBeenCalledWith({ name: 'Zatoka Pucka' });
	});

	it('notes a failed suggestion request instead of showing no suggestions', async () => {
		suggestMock.mockRejectedValue(new ApiError(500, 'boom'));
		render(PortPicker, { props: { ports: [] } });
		await typeQuery('he');
		expect(
			await screen.findByText('Nie udało się pobrać wcześniej odwiedzonych portów')
		).toBeInTheDocument();
	});

	it('shows suggestions and geocoder results together, with OSM attribution', async () => {
		suggestMock.mockResolvedValue([
			{ name: 'Jastarnia', latitude: 54.7, longitude: 18.68, uses: 1 }
		]);
		geocodeMock.mockResolvedValue([
			{
				name: 'Jastarnia',
				label: 'Jastarnia, Puck County, Pomeranian Voivodeship, Poland',
				latitude: 54.69,
				longitude: 18.67,
				source: 'photon'
			}
		]);
		render(PortPicker, { props: { ports: [] } });
		await typeQuery('Jastarn');
		await screen.findByText('Wcześniej odwiedzone');
		await fireEvent.click(screen.getByRole('button', { name: 'Szukaj' }));
		expect(await screen.findByText('Wyniki wyszukiwania')).toBeInTheDocument();
		expect(
			screen.getByText('Jastarnia, Puck County, Pomeranian Voivodeship, Poland')
		).toBeInTheDocument();
		expect(screen.getByText('Wcześniej odwiedzone')).toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'współtwórcy OpenStreetMap' })).toHaveAttribute(
			'href',
			'https://www.openstreetmap.org/copyright'
		);
		expect(screen.getByText(/Photon \(komoot\)/)).toBeInTheDocument();
	});

	it.each([
		[
			'busy',
			new ApiError(503, 'geocoding service busy, try again'),
			'Wyszukiwarka jest chwilowo zajęta, spróbuj za chwilę'
		],
		[
			'failed upstream',
			new ApiError(502, 'geocoding service unavailable'),
			'Nie udało się wyszukać miejsca'
		],
		['network failure', new TypeError('Failed to fetch'), 'Nie udało się wyszukać miejsca']
	])('tells a %s search apart', async (_, err, message) => {
		geocodeMock.mockRejectedValue(err);
		render(PortPicker, { props: { ports: [] } });
		await typeQuery('Split');
		await fireEvent.click(screen.getByRole('button', { name: 'Szukaj' }));
		expect(await screen.findByText(message)).toBeInTheDocument();
	});

	it('says when the geocoder finds nothing', async () => {
		geocodeMock.mockResolvedValue([]);
		render(PortPicker, { props: { ports: [] } });
		await typeQuery('Zzzz');
		await fireEvent.click(screen.getByRole('button', { name: 'Szukaj' }));
		expect(await screen.findByText('Brak wyników')).toBeInTheDocument();
	});

	it('sends the map centre as bias once the map is ready', async () => {
		geocodeMock.mockResolvedValue([]);
		const { container } = render(PortPicker, { props: { ports: [] } });
		await waitFor(() => expect(container.querySelector('.leaflet-container')).not.toBeNull());
		await typeQuery('Hel');
		await fireEvent.click(screen.getByRole('button', { name: 'Szukaj' }));
		await waitFor(() => expect(geocodeMock).toHaveBeenCalledTimes(1));
		expect(geocodeMock).toHaveBeenCalledWith('Hel', { lat: 54, lon: 15 });
	});

	const clickMap = async (container: HTMLElement) => {
		await waitFor(() => expect(container.querySelector('.leaflet-container')).not.toBeNull());
		await fireEvent.click(screen.getByLabelText('Mapa portów'), { clientX: 10, clientY: 10 });
	};

	it('adds a point clicked on the map under the typed name', async () => {
		const onAdd = vi.fn();
		const { container } = render(PortPicker, { props: { ports: [], onAdd } });
		await typeQuery('Port Leba');
		await clickMap(container);

		const name = await screen.findByLabelText('Nazwa punktu z mapy');
		expect(name).toHaveValue('Port Leba');
		await fireEvent.input(name, { target: { value: 'Łeba' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj punkt' }));

		expect(onAdd).toHaveBeenCalledTimes(1);
		const added = onAdd.mock.calls[0][0];
		expect(added.name).toBe('Łeba');
		expect(Number.isFinite(added.latitude)).toBe(true);
		expect(Number.isFinite(added.longitude)).toBe(true);
		expect(Math.abs(added.longitude)).toBeLessThanOrEqual(180);
		await waitFor(() => expect(screen.queryByLabelText('Nazwa punktu z mapy')).toBeNull());
		expect(geocodeMock).not.toHaveBeenCalled();
	});

	it('needs a name for a map point and can drop it', async () => {
		const onAdd = vi.fn();
		const { container } = render(PortPicker, { props: { ports: [], onAdd } });
		await clickMap(container);
		await screen.findByLabelText('Nazwa punktu z mapy');
		expect(screen.getByRole('button', { name: 'Dodaj punkt' })).toBeDisabled();
		await fireEvent.click(screen.getByRole('button', { name: 'Anuluj' }));
		expect(screen.queryByLabelText('Nazwa punktu z mapy')).toBeNull();
		expect(onAdd).not.toHaveBeenCalled();
	});

	it('ignores map clicks when readonly', async () => {
		const { container } = render(PortPicker, {
			props: { ports: [], readonly: true }
		});
		await clickMap(container);
		expect(screen.queryByLabelText('Nazwa punktu z mapy')).toBeNull();
	});

	it('adds a port by name only', async () => {
		const onAdd = vi.fn();
		render(PortPicker, { props: { ports: [], onAdd } });
		await typeQuery('Zatoka Gdańska');
		await fireEvent.click(screen.getByRole('button', { name: 'Dodaj bez lokalizacji na mapie' }));
		expect(onAdd).toHaveBeenCalledWith({ name: 'Zatoka Gdańska' });
		expect(geocodeMock).not.toHaveBeenCalled();
	});

	it('disables Szukaj for a query shorter than two characters', async () => {
		render(PortPicker, { props: { ports: [] } });
		await typeQuery('S');
		expect(screen.getByRole('button', { name: 'Szukaj' })).toBeDisabled();
		await fireEvent.keyDown(screen.getByLabelText('Odwiedzone porty'), { key: 'Enter' });
		expect(geocodeMock).not.toHaveBeenCalled();
	});

	it('clears stale results when the query changes', async () => {
		geocodeMock.mockResolvedValue([{ name: 'Split', latitude: 43.5, longitude: 16.4 }]);
		render(PortPicker, { props: { ports: [] } });
		await typeQuery('Split');
		await fireEvent.click(screen.getByRole('button', { name: 'Szukaj' }));
		await screen.findByRole('button', { name: 'Split' });

		await typeQuery('Hvar');
		expect(screen.queryByRole('button', { name: 'Split' })).toBeNull();
	});

	it('renders a port name in the map tooltip as text, not HTML', async () => {
		const { container } = render(PortPicker, {
			props: {
				ports: [{ name: '<img src=x onerror=alert(1)>', latitude: 43.5, longitude: 16.4 }],
				readonly: true
			}
		});
		await waitFor(() => expect(container.querySelector('.leaflet-tooltip')).not.toBeNull());
		const tooltip = container.querySelector('.leaflet-tooltip')!;
		expect(tooltip.querySelector('img')).toBeNull();
		expect(tooltip.textContent).toBe('1. <img src=x onerror=alert(1)>');
	});

	it('calls onRemove with the port index', async () => {
		const onRemove = vi.fn();
		render(PortPicker, {
			props: { ports: [{ name: 'Split', latitude: 43.5, longitude: 16.4 }], onRemove }
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Usuń' }));
		expect(onRemove).toHaveBeenCalledWith(0);
	});

	it('hides search and remove controls when readonly', () => {
		render(PortPicker, {
			props: { ports: [{ name: 'Split', latitude: 43.5, longitude: 16.4 }], readonly: true }
		});
		expect(screen.queryByLabelText('Odwiedzone porty')).toBeNull();
		expect(screen.queryByRole('button', { name: 'Usuń' })).toBeNull();
	});

	const threePorts = [
		{ name: 'Split', latitude: 43.5, longitude: 16.4 },
		{ name: 'Hvar', latitude: 43.1, longitude: 16.4 },
		{ name: 'Vis', latitude: 43.0, longitude: 16.2 }
	];

	it('shows no reorder controls without onReorder', () => {
		render(PortPicker, { props: { ports: threePorts } });
		expect(screen.queryByRole('button', { name: /w dół$/ })).toBeNull();
	});

	it('moves a port down via the keyboard button', async () => {
		const onReorder = vi.fn();
		render(PortPicker, { props: { ports: threePorts, onReorder } });
		const down = screen.getAllByRole('button', { name: /w dół$/ });
		await fireEvent.click(down[0]);
		expect(onReorder).toHaveBeenCalledWith(0, 1);
	});

	it('moves a port up via the keyboard button', async () => {
		const onReorder = vi.fn();
		render(PortPicker, { props: { ports: threePorts, onReorder } });
		const up = screen.getAllByRole('button', { name: /w górę$/ });
		await fireEvent.click(up[2]);
		expect(onReorder).toHaveBeenCalledWith(2, 1);
	});

	it('disables up on the first port and down on the last', () => {
		render(PortPicker, { props: { ports: threePorts, onReorder: vi.fn() } });
		const up = screen.getAllByRole('button', { name: /w górę$/ });
		const down = screen.getAllByRole('button', { name: /w dół$/ });
		expect(up[0]).toBeDisabled();
		expect(down[down.length - 1]).toBeDisabled();
	});

	it('reorders via drag and drop', async () => {
		const onReorder = vi.fn();
		render(PortPicker, { props: { ports: threePorts, onReorder } });
		const rows = screen.getAllByRole('listitem');
		await fireEvent.dragStart(rows[0]);
		await fireEvent.drop(rows[2]);
		expect(onReorder).toHaveBeenCalledWith(0, 2);
	});

	it('ignores a drop onto the same port', async () => {
		const onReorder = vi.fn();
		render(PortPicker, { props: { ports: threePorts, onReorder } });
		const rows = screen.getAllByRole('listitem');
		await fireEvent.dragStart(rows[1]);
		await fireEvent.drop(rows[1]);
		expect(onReorder).not.toHaveBeenCalled();
	});
});
