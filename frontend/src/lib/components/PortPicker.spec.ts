import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import PortPicker from './PortPicker.svelte';

const geocodeMock = vi.fn();
vi.mock('#lib/api/routes.js', () => ({ geocode: (q: string) => geocodeMock(q) }));

describe('PortPicker', () => {
	beforeEach(() => geocodeMock.mockReset());

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
		expect(geocodeMock).toHaveBeenCalledWith('Split');
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
			expect(geocodeMock).toHaveBeenCalledWith('Hvar');
			expect(notCancelled).toBe(false);
			expect(onSubmit).not.toHaveBeenCalled();
		} finally {
			form.remove();
		}
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
