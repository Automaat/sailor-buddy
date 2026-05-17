import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import PortPicker from './PortPicker.svelte';

const geocodeMock = vi.fn();
vi.mock('$lib/api/routes', () => ({ geocode: (q: string) => geocodeMock(q) }));

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

	it('searches by town name and adds the picked result', async () => {
		geocodeMock.mockResolvedValue([{ name: 'Split', latitude: 43.5, longitude: 16.4 }]);
		const onAdd = vi.fn();
		// debounceMs 0 keeps the search deterministic without the 400ms wait.
		render(PortPicker, { props: { ports: [], onAdd, debounceMs: 0 } });

		await fireEvent.input(screen.getByLabelText('Odwiedzone porty'), {
			target: { value: 'Split' }
		});
		const result = await screen.findByRole('button', { name: 'Split' });
		await fireEvent.click(result);

		expect(geocodeMock).toHaveBeenCalledWith('Split');
		expect(onAdd).toHaveBeenCalledWith({ name: 'Split', latitude: 43.5, longitude: 16.4 });
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
});
