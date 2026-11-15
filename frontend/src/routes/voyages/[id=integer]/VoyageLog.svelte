<script lang="ts">
	import { updateVoyageLog, addVoyageYacht, listVoyageYachts } from '#lib/api/routes.js';
	import type { Yacht, YachtBody, YachtSummary } from '#lib/api/types.js';
	import { errorMessage } from '#lib/api/errors.js';
	import YachtSelect from '#lib/components/YachtSelect.svelte';

	import { onDestroy } from 'svelte';
	import type { PageData } from './$types';
	let active = true;
	onDestroy(() => {
		active = false;
	});
	function leftVoyage(voyageId: number) {
		return !active || voyageId !== id;
	}
	let {
		voyage = $bindable(),
		yacht = $bindable(),
		canLog
	}: { voyage: PageData['voyage']; yacht: YachtSummary | null; canLog: boolean } = $props();
	const id = $derived(voyage.id);

	// The picker's list comes through the voyage: its captain may be outside
	// the captain group, which alone reads the club yacht list.
	let yachts = $state<Yacht[]>([]);
	let yachtsError = $state('');
	$effect(() => {
		if (!canLog) return;
		const voyageId = id;
		yachtsError = '';
		listVoyageYachts(voyageId).then(
			(list) => {
				if (leftVoyage(voyageId)) return;
				// Keep a yacht added from the log before the list answered.
				yachts = [...list, ...yachts.filter((y) => !list.some((l) => l.id === y.id))];
			},
			(err) => {
				if (!leftVoyage(voyageId)) yachtsError = errorMessage(err);
			}
		);
	});
	let logOpen = $state(false);
	let logError = $state('');
	let logForm = $state({
		hours_total: 0,
		hours_sail: 0,
		hours_engine: 0,
		hours_over_6bf: 0,
		miles: 0,
		days: 0,
		tidal_waters: false,
		captain_notes: '',
		yacht_id: 0,
		track_url: '',
		voyage_card: ''
	});

	let logSavingFor = $state<number | null>(null);
	const logSaving = $derived(logSavingFor === id);

	function openLog() {
		logForm = {
			hours_total: voyage.hours_total,
			hours_sail: voyage.hours_sail,
			hours_engine: voyage.hours_engine,
			hours_over_6bf: voyage.hours_over_6bf,
			miles: voyage.miles,
			days: voyage.days,
			tidal_waters: !!voyage.tidal_waters,
			captain_notes: voyage.captain_notes ?? '',
			yacht_id: voyage.yacht_id ?? 0,
			track_url: voyage.track_url ?? '',
			voyage_card: voyage.voyage_card ?? ''
		};
		logError = '';
		logOpen = true;
	}

	async function saveLog(e: Event) {
		e.preventDefault();
		const voyageId = id;
		logSavingFor = voyageId;
		logError = '';
		try {
			const body = {
				...logForm,
				tidal_waters: logForm.tidal_waters ? 1 : 0,
				captain_notes: logForm.captain_notes || undefined,
				yacht_id: logForm.yacht_id || undefined,
				track_url: logForm.track_url.trim() || undefined,
				voyage_card: logForm.voyage_card.trim() || undefined
			};
			await updateVoyageLog(voyageId, body);
			if (leftVoyage(voyageId)) return;
			voyage = { ...voyage, ...body };
			yacht = body.yacht_id ? summaryOf(body.yacht_id) : null;
			logOpen = false;
		} catch (err) {
			if (leftVoyage(voyageId)) return;
			logError = err instanceof Error ? err.message : 'Nie udało się zapisać dziennika';
		} finally {
			if (logSavingFor === voyageId) logSavingFor = null;
		}
	}

	// summaryOf names the logged yacht; one missing from a list that failed to
	// load is the voyage's unchanged yacht.
	function summaryOf(yachtID: number): YachtSummary | null {
		const picked = yachts.find((y) => y.id === yachtID);
		if (picked) return { id: picked.id, name: picked.name, yacht_type: picked.yacht_type };
		return yacht?.id === yachtID ? yacht : null;
	}

	function addYachtForVoyage(body: YachtBody) {
		return addVoyageYacht(id, body);
	}
</script>

{#if canLog || voyage.captain_notes}
	<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
		<div class="mb-3 flex items-center justify-between">
			<h2 class="font-semibold text-[var(--navy)]">Raport kapitana</h2>
			{#if canLog && !logOpen}
				<button onclick={openLog} class="rounded-lg border px-3 py-1 text-sm hover:bg-gray-50">
					Uzupełnij po rejsie
				</button>
			{/if}
		</div>
		{#if logOpen}
			{#if logError}
				<div class="mb-3 rounded-lg bg-red-50 p-3 text-sm text-red-600">{logError}</div>
			{/if}
			<form onsubmit={saveLog} class="space-y-4">
				<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
					<div>
						<label for="log_hours_total" class="mb-1 block text-xs font-medium"
							>Godziny łącznie</label
						>
						<input
							id="log_hours_total"
							type="number"
							step="0.1"
							min="0"
							bind:value={logForm.hours_total}
							class="w-full rounded-lg border px-3 py-2"
						/>
					</div>
					<div>
						<label for="log_hours_sail" class="mb-1 block text-xs font-medium"
							>Godziny na żaglach</label
						>
						<input
							id="log_hours_sail"
							type="number"
							step="0.1"
							min="0"
							bind:value={logForm.hours_sail}
							class="w-full rounded-lg border px-3 py-2"
						/>
					</div>
					<div>
						<label for="log_hours_engine" class="mb-1 block text-xs font-medium"
							>Godziny na silniku</label
						>
						<input
							id="log_hours_engine"
							type="number"
							step="0.1"
							min="0"
							bind:value={logForm.hours_engine}
							class="w-full rounded-lg border px-3 py-2"
						/>
					</div>
					<div>
						<label for="log_hours_6bf" class="mb-1 block text-xs font-medium">Godziny &gt;6Bf</label
						>
						<input
							id="log_hours_6bf"
							type="number"
							step="0.1"
							min="0"
							bind:value={logForm.hours_over_6bf}
							class="w-full rounded-lg border px-3 py-2"
						/>
					</div>
					<div>
						<label for="log_miles" class="mb-1 block text-xs font-medium">Mile</label>
						<input
							id="log_miles"
							type="number"
							step="0.1"
							min="0"
							bind:value={logForm.miles}
							class="w-full rounded-lg border px-3 py-2"
						/>
					</div>
					<div>
						<label for="log_days" class="mb-1 block text-xs font-medium">Dni</label>
						<input
							id="log_days"
							type="number"
							min="0"
							bind:value={logForm.days}
							class="w-full rounded-lg border px-3 py-2"
						/>
					</div>
					<label class="col-span-2 flex items-center gap-2 text-sm">
						<input type="checkbox" bind:checked={logForm.tidal_waters} />
						Wody pływowe
					</label>
				</div>
				<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
					<div class="sm:col-span-2">
						<label for="log_yacht" class="mb-1 block text-xs font-medium">Jacht</label>
						<YachtSelect
							id="log_yacht"
							bind:value={logForm.yacht_id}
							bind:yachts
							create={addYachtForVoyage}
						/>
						{#if yachtsError}
							<p role="alert" class="mt-1 text-xs text-red-600">
								Nie udało się wczytać listy jachtów: {yachtsError}
							</p>
						{/if}
					</div>
					<div>
						<label for="log_track_url" class="mb-1 block text-xs font-medium"
							>Link do śladu rejsu</label
						>
						<input
							id="log_track_url"
							type="url"
							bind:value={logForm.track_url}
							placeholder="https://..."
							class="w-full rounded-lg border px-3 py-2"
						/>
					</div>
					<div>
						<label for="log_voyage_card" class="mb-1 block text-xs font-medium">Karta rejsu</label>
						<input
							id="log_voyage_card"
							type="text"
							bind:value={logForm.voyage_card}
							placeholder="np. u kapitana, w klubie"
							class="w-full rounded-lg border px-3 py-2"
						/>
					</div>
				</div>
				<div>
					<label for="log_notes" class="mb-1 block text-xs font-medium">Notatka z rejsu</label>
					<textarea
						id="log_notes"
						rows="4"
						bind:value={logForm.captain_notes}
						class="w-full rounded-lg border px-3 py-2"></textarea>
				</div>
				<p class="text-xs text-[var(--text-muted)]">
					Odwiedzone mariny dodasz w sekcji „Odwiedzone porty” poniżej.
				</p>
				<div class="flex gap-2">
					<button
						type="submit"
						disabled={logSaving}
						class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
					>
						{logSaving ? 'Zapisywanie...' : 'Zapisz raport'}
					</button>
					<button
						type="button"
						onclick={() => (logOpen = false)}
						class="rounded-lg border px-4 py-2 text-sm text-[var(--text-muted)] hover:bg-gray-50"
					>
						Anuluj
					</button>
				</div>
			</form>
		{:else if voyage.captain_notes}
			<p class="whitespace-pre-wrap text-sm">{voyage.captain_notes}</p>
		{:else}
			<p class="text-sm text-[var(--text-muted)]">Kapitan nie dodał jeszcze raportu.</p>
		{/if}
	</div>
{/if}
