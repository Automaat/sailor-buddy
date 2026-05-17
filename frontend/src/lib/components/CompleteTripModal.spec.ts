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

async function submit() {
	await fireEvent.click(screen.getByRole('button', { name: 'Zrealizuj rejs' }));
}

describe('CompleteTripModal', () => {
	it('renders the trip name', () => {
		setup();
		expect(screen.getByText('Rejs Bałtyk')).toBeInTheDocument();
	});

	it('derives the year from the embark date', async () => {
		const { onSubmit } = setup({ embarkDate: '2024-08-12' });
		await submit();
		await waitFor(() => expect(onSubmit.mock.calls[0][0]).toMatchObject({ year: 2024 }));
	});

	it('falls back to the current year without an embark date', async () => {
		const { onSubmit } = setup({ embarkDate: null });
		await submit();
		await waitFor(() =>
			expect(onSubmit.mock.calls[0][0]).toMatchObject({ year: new Date().getFullYear() })
		);
	});

	it('derives an inclusive day count between the two dates', async () => {
		const { onSubmit } = setup({ embarkDate: '2025-06-01', disembarkDate: '2025-06-07' });
		await submit();
		await waitFor(() => expect(onSubmit.mock.calls[0][0]).toMatchObject({ days: 7 }));
	});

	it('omits days when disembark precedes embark', async () => {
		const { onSubmit } = setup({ embarkDate: '2025-06-07', disembarkDate: '2025-06-01' });
		await submit();
		await waitFor(() => expect(onSubmit).toHaveBeenCalled());
		expect(onSubmit.mock.calls[0][0].days).toBeUndefined();
	});

	it('derives total hours from sail and engine hours', async () => {
		const { onSubmit } = setup();
		await fireEvent.input(screen.getByLabelText('Godziny żagli'), { target: { value: '10' } });
		await fireEvent.input(screen.getByLabelText('Godziny silnika'), { target: { value: '5' } });
		await submit();
		await waitFor(() => expect(onSubmit.mock.calls[0][0]).toMatchObject({ hours_total: 15 }));
	});

	it('submits a payload with computed and entered values', async () => {
		const { onSubmit } = setup();
		await fireEvent.input(screen.getByLabelText('Godziny żagli'), { target: { value: '10' } });
		await fireEvent.input(screen.getByLabelText('Godziny silnika'), { target: { value: '5' } });
		await fireEvent.input(screen.getByLabelText('Mile'), { target: { value: '120' } });
		await fireEvent.click(screen.getByLabelText('Wody pływowe'));
		await submit();

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
		await submit();
		await waitFor(() => expect(onSubmit).toHaveBeenCalled());
		expect(onSubmit.mock.calls[0][0]).toMatchObject({ tidal_waters: 0 });
	});

	it('shows an error message when submission fails', async () => {
		const onSubmit = vi.fn().mockRejectedValue(new Error('Serwer padł'));
		setup({ onSubmit });
		await submit();
		expect(await screen.findByText('Serwer padł')).toBeInTheDocument();
	});

	it('calls onClose when cancel is clicked', async () => {
		const { onClose } = setup();
		await fireEvent.click(screen.getByRole('button', { name: 'Anuluj' }));
		expect(onClose).toHaveBeenCalledOnce();
	});
});
