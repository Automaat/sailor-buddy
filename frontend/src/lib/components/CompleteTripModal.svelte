<script lang="ts">
	import type { CompleteTripPayload, VoyagePortBody } from '$lib/api/types';
	import PortPicker from './PortPicker.svelte';

	type Props = {
		tripName: string;
		embarkDate?: string | null;
		disembarkDate?: string | null;
		onClose: () => void;
		onSubmit: (payload: CompleteTripPayload) => Promise<void>;
	};

	let { tripName, embarkDate, disembarkDate, onClose, onSubmit }: Props = $props();

	function yearFromDate(d?: string | null): number {
		if (!d) return new Date().getFullYear();
		const y = parseInt(d.split('-')[0], 10);
		return Number.isFinite(y) ? y : new Date().getFullYear();
	}

	function daysBetween(start?: string | null, end?: string | null): number {
		if (!start || !end) return 0;
		const s = Date.parse(start);
		const e = Date.parse(end);
		if (!Number.isFinite(s) || !Number.isFinite(e) || e < s) return 0;
		return Math.round((e - s) / 86_400_000) + 1;
	}

	const computedYear = $derived(yearFromDate(embarkDate));
	const computedDays = $derived(daysBetween(embarkDate, disembarkDate));

	let form = $state({
		hours_sail: 0,
		hours_engine: 0,
		hours_over_6bf: 0,
		miles: 0,
		tidal_waters: false
	});
	const hoursTotal = $derived((form.hours_sail || 0) + (form.hours_engine || 0));
	let ports = $state<VoyagePortBody[]>([]);
	let submitting = $state(false);
	let error = $state('');

	function addPort(port: VoyagePortBody) {
		ports = [...ports, { ...port, position: ports.length }];
	}

	function removePort(index: number) {
		ports = ports.filter((_, i) => i !== index).map((p, i) => ({ ...p, position: i }));
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		submitting = true;
		error = '';
		try {
			await onSubmit({
				year: computedYear || undefined,
				hours_total: hoursTotal || undefined,
				hours_sail: form.hours_sail || undefined,
				hours_engine: form.hours_engine || undefined,
				hours_over_6bf: form.hours_over_6bf || undefined,
				miles: form.miles || undefined,
				days: computedDays || undefined,
				tidal_waters: form.tidal_waters ? 1 : 0,
				ports: ports.length > 0 ? ports : undefined
			});
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się zrealizować rejsu';
		} finally {
			submitting = false;
		}
	}
</script>

<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" role="dialog" aria-modal="true">
	<div class="max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-2xl bg-white p-6 shadow-xl">
		<h2 class="mb-1 text-xl font-semibold text-[var(--navy)]">Zrealizuj rejs</h2>
		<p class="mb-4 text-sm text-[var(--text-muted)]">{tripName}</p>

		{#if error}
			<div class="mb-3 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
		{/if}

		<form onsubmit={handleSubmit} class="space-y-4">
			<div class="grid grid-cols-2 gap-3 md:grid-cols-4">
				<div>
					<label for="m-miles" class="mb-1 block text-xs font-medium">Mile</label>
					<input id="m-miles" type="number" step="0.1" bind:value={form.miles} class="w-full rounded-lg border px-2 py-1.5 text-sm" />
				</div>
				<div class="flex items-end">
					<label class="flex items-center gap-2 text-sm">
						<input type="checkbox" bind:checked={form.tidal_waters} />
						Wody pływowe
					</label>
				</div>
				<div>
					<label for="m-hs" class="mb-1 block text-xs font-medium">Godziny żagli</label>
					<input id="m-hs" type="number" step="0.1" bind:value={form.hours_sail} class="w-full rounded-lg border px-2 py-1.5 text-sm" />
				</div>
				<div>
					<label for="m-he" class="mb-1 block text-xs font-medium">Godziny silnika</label>
					<input id="m-he" type="number" step="0.1" bind:value={form.hours_engine} class="w-full rounded-lg border px-2 py-1.5 text-sm" />
				</div>
				<div>
					<label for="m-h6" class="mb-1 block text-xs font-medium">Godziny &gt;6Bf</label>
					<input id="m-h6" type="number" step="0.1" bind:value={form.hours_over_6bf} class="w-full rounded-lg border px-2 py-1.5 text-sm" />
				</div>
			</div>

			<PortPicker {ports} onAdd={addPort} onRemove={removePort} />

			<div class="flex justify-end gap-2 pt-2">
				<button type="button" onclick={onClose} class="rounded-lg border px-4 py-2 text-sm text-[var(--text-muted)] hover:bg-gray-50">
					Anuluj
				</button>
				<button type="submit" disabled={submitting} class="rounded-lg bg-green-600 px-4 py-2 text-sm font-medium text-white hover:bg-green-700 disabled:opacity-50">
					{submitting ? 'Zapisywanie...' : 'Zrealizuj rejs'}
				</button>
			</div>
		</form>
	</div>
</div>
