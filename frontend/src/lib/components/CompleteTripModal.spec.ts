import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import type { CompleteTripPayload } from '$lib/api/types';
import CompleteTripModal from './CompleteTripModal.svelte';

type Props = {
	tripName: string;
	embarkDate?: string | null;
	disembarkDate?: string | null;
	onClose: () => void;
	onSubmit: (payload: CompleteTripPayload) => Promise<void>;
};

function setup(overrides: Partial<Props> = {}) {
	const onClose = vi.fn();
	const onSubmit = vi.fn().mockResolvedValue(undefined);
	const result = render(CompleteTripModal, {
		props: {
			tripName: 'Rejs Bałtyk',
			embarkDate: '2025-06-01',
			disembarkDate: '2025-06-07',
			onClose,
			onSubmit,
			...overrides
		}
	});
	return { ...result, onClose, onSubmit };
}

const value = (label: string) => (screen.getByLabelText(label) as HTMLInputElement).value;

describe('CompleteTripModal', () => {
	it('renders the trip name', () => {
		setup();
		expect(screen.getByText('Rejs Bałtyk')).toBeInTheDocument();
	});

	it('derives the year from the embark date', () => {
		setup({ embarkDate: '2024-08-12' });
		expect(value('Rok')).toBe('2024');
	});

	it('falls back to the current year without an embark date', () => {
		setup({ embarkDate: null });
		expect(value('Rok')).toBe(String(new Date().getFullYear()));
	});

	it('derives an inclusive day count between the two dates', () => {
		setup({ embarkDate: '2025-06-01', disembarkDate: '2025-06-07' });
		expect(value('Dni')).toBe('7');
	});

	it('shows zero days when disembark precedes embark', () => {
		setup({ embarkDate: '2025-06-07', disembarkDate: '2025-06-01' });
		expect(value('Dni')).toBe('0');
	});

	it('derives total hours from sail and engine hours', async () => {
		setup();
		await fireEvent.input(screen.getByLabelText('Godziny żagli'), { target: { value: '10' } });
		await fireEvent.input(screen.getByLabelText('Godziny silnika'), { target: { value: '5' } });
		expect(value('Godziny łącznie')).toBe('15');
	});

	it('submits a payload with computed and entered values', async () => {
		const { onSubmit } = setup();
		await fireEvent.input(screen.getByLabelText('Godziny żagli'), { target: { value: '10' } });
		await fireEvent.input(screen.getByLabelText('Godziny silnika'), { target: { value: '5' } });
		await fireEvent.input(screen.getByLabelText('Mile'), { target: { value: '120' } });
		await fireEvent.click(screen.getByLabelText('Wody pływowe'));
		await fireEvent.click(screen.getByRole('button', { name: 'Zrealizuj rejs' }));

		await waitFor(() =>
			expect(onSubmit).toHaveBeenCalledWith({
				year: 2025,
				hours_total: 15,
				hours_sail: 10,
				hours_engine: 5,
				hours_over_6bf: undefined,
				miles: 120,
				days: 7,
				tidal_waters: 1
			})
		);
	});

	it('sends tidal_waters as 0 when the checkbox is unchecked', async () => {
		const { onSubmit } = setup();
		await fireEvent.click(screen.getByRole('button', { name: 'Zrealizuj rejs' }));
		await waitFor(() => expect(onSubmit).toHaveBeenCalled());
		expect(onSubmit.mock.calls[0][0]).toMatchObject({ tidal_waters: 0 });
	});

	it('shows an error message when submission fails', async () => {
		const onSubmit = vi.fn().mockRejectedValue(new Error('Serwer padł'));
		setup({ onSubmit });
		await fireEvent.click(screen.getByRole('button', { name: 'Zrealizuj rejs' }));
		expect(await screen.findByText('Serwer padł')).toBeInTheDocument();
	});

	it('calls onClose when cancel is clicked', async () => {
		const { onClose } = setup();
		await fireEvent.click(screen.getByRole('button', { name: 'Anuluj' }));
		expect(onClose).toHaveBeenCalledOnce();
	});
});
