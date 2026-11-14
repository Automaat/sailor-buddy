<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { updateVoyage } from '#lib/api/routes.js';
	import type { Voyage } from '#lib/api/types.js';
	import { goto } from '$app/navigation';
	import CaptainSelect from '#lib/components/CaptainSelect.svelte';
	import CruiseSelect from '#lib/components/CruiseSelect.svelte';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import YachtSelect from '#lib/components/YachtSelect.svelte';
	import { perLoad } from '#lib/state.svelte.js';

	let { data } = $props();

	const id = $derived(data.voyage.id);
	const cruises = $derived(data.cruises);
	const loadErrors = $derived(data.loadErrors);
	let yachts = $derived(data.yachts);
	const legacyCaptainName = $derived(data.voyage.captain_person_id ? '' : (data.voyage.captain_name ?? ''));

	let form = $derived(perLoad(data, voyageForm(data.voyage)));
	let submit = $derived(perLoad(data, { error: '', saving: false }));

	// Editing voyages is an admin task; redirect regular members away.
	$effect(() => {
		if (auth.user && !auth.isAdmin) {
			void goto(`/voyages/${id}`);
		}
	});

	function voyageForm(voyage: Voyage) {
		return {
			name: voyage.name,
			year: voyage.year ?? 0,
			embark_date: voyage.embark_date ?? '',
			disembark_date: voyage.disembark_date ?? '',
			countries: voyage.countries ?? '',
			start_port: voyage.start_port ?? '',
			end_port: voyage.end_port ?? '',
			cruise_id: voyage.cruise_id,
			hours_total: voyage.hours_total,
			hours_sail: voyage.hours_sail,
			hours_engine: voyage.hours_engine,
			hours_over_6bf: voyage.hours_over_6bf,
			miles: voyage.miles,
			days: voyage.days,
			captain_person_id: voyage.captain_person_id ?? 0,
			yacht_id: voyage.yacht_id ?? 0,
			tidal_waters: voyage.tidal_waters > 0,
			cost_total: voyage.cost_total ?? 0,
			cost_per_person: voyage.cost_per_person ?? 0,
			description: voyage.description ?? '',
			image_logo_url: voyage.image_logo_url ?? '',
			image_photo_url: voyage.image_photo_url ?? '',
			image_route_url: voyage.image_route_url ?? '',
			track_url: voyage.track_url ?? '',
			voyage_card: voyage.voyage_card ?? ''
		};
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		const run = submit;
		if (!form.cruise_id) {
			run.error = 'Wybierz wydarzenie, do którego należy rejs';
			return;
		}
		const voyageId = id;
		run.saving = true;
		run.error = '';
		try {
			const payload = {
				...form,
				tidal_waters: form.tidal_waters ? 1 : 0,
				yacht_id: form.yacht_id || undefined,
				image_logo_url: form.image_logo_url || undefined,
				image_photo_url: form.image_photo_url || undefined,
				image_route_url: form.image_route_url || undefined,
				track_url: form.track_url.trim() || undefined,
				voyage_card: form.voyage_card.trim() || undefined,
				// captain_name of a linked captain is the server's snapshot, so only a
				// legacy free-text name survives clearing the captain.
				captain_person_id: form.captain_person_id || undefined,
				captain_name: form.captain_person_id ? undefined : legacyCaptainName || undefined
			};
			await updateVoyage(voyageId, payload);
			if (run === submit) void goto(`/voyages/${voyageId}`);
		} catch (err) {
			run.error = err instanceof Error ? err.message : 'Nie udało się zapisać';
		} finally {
			run.saving = false;
		}
	}
</script>

<div class="mx-auto max-w-3xl">
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Edytuj rejs</h1>
	<LoadErrorBanner errors={loadErrors} />

	{#if submit.error}
		<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{submit.error}</div>
	{/if}

	<!-- Remounts the pickers for each voyage, so a yacht still being added for
	the previous voyage is never picked, or its held save replayed, on this one. -->
	{#key data}
		<form onsubmit={handleSubmit} class="space-y-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
				<div class="sm:col-span-2">
					<label for="cruise" class="mb-1 block text-sm font-medium">Wydarzenie klubu *</label>
					<CruiseSelect bind:value={form.cruise_id} {cruises} />
				</div>
				<div class="sm:col-span-2">
					<label for="name" class="mb-1 block text-sm font-medium">Nazwa rejsu *</label>
					<input id="name" type="text" bind:value={form.name} required class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="year" class="mb-1 block text-sm font-medium">Rok</label>
					<input id="year" type="number" bind:value={form.year} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="yacht" class="mb-1 block text-sm font-medium">Jacht</label>
					<YachtSelect bind:value={form.yacht_id} bind:yachts />
				</div>
				<div>
					<label for="embark" class="mb-1 block text-sm font-medium">Data zaokrętowania</label>
					<input id="embark" type="date" bind:value={form.embark_date} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="disembark" class="mb-1 block text-sm font-medium">Data wyokrętowania</label>
					<input id="disembark" type="date" bind:value={form.disembark_date} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="start_port" class="mb-1 block text-sm font-medium">Port wyjścia</label>
					<input id="start_port" type="text" bind:value={form.start_port} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="end_port" class="mb-1 block text-sm font-medium">Port docelowy</label>
					<input id="end_port" type="text" bind:value={form.end_port} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="countries" class="mb-1 block text-sm font-medium">Kraje</label>
					<input id="countries" type="text" bind:value={form.countries} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="captain" class="mb-1 block text-sm font-medium">Kapitan</label>
					<CaptainSelect bind:value={form.captain_person_id} legacyName={legacyCaptainName} />
				</div>
			</div>

			<hr />
			<h3 class="font-semibold text-[var(--navy)]">Statystyki nawigacyjne</h3>
			<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
				<div>
					<label for="ht" class="mb-1 block text-sm font-medium">Godziny łącznie</label>
					<input id="ht" type="number" step="0.1" bind:value={form.hours_total} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="hs" class="mb-1 block text-sm font-medium">Godziny żagli</label>
					<input id="hs" type="number" step="0.1" bind:value={form.hours_sail} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="he" class="mb-1 block text-sm font-medium">Godziny silnika</label>
					<input id="he" type="number" step="0.1" bind:value={form.hours_engine} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="h6" class="mb-1 block text-sm font-medium">Godziny &gt;6Bf</label>
					<input id="h6" type="number" step="0.1" bind:value={form.hours_over_6bf} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="mi" class="mb-1 block text-sm font-medium">Mile</label>
					<input id="mi" type="number" step="0.1" bind:value={form.miles} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="da" class="mb-1 block text-sm font-medium">Dni</label>
					<input id="da" type="number" bind:value={form.days} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div class="flex items-end">
					<label class="flex items-center gap-2 text-sm">
						<input type="checkbox" bind:checked={form.tidal_waters} />
						Wody pływowe
					</label>
				</div>
			</div>

			<hr />
			<h3 class="font-semibold text-[var(--navy)]">Koszty</h3>
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
				<div>
					<label for="ct" class="mb-1 block text-sm font-medium">Koszt całkowity</label>
					<input id="ct" type="number" step="0.01" bind:value={form.cost_total} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="cp" class="mb-1 block text-sm font-medium">Koszt na osobę</label>
					<input id="cp" type="number" step="0.01" bind:value={form.cost_per_person} class="w-full rounded-lg border px-3 py-2" />
				</div>
			</div>

			<div>
				<label for="desc" class="mb-1 block text-sm font-medium">Opis</label>
				<textarea id="desc" bind:value={form.description} rows="4" class="w-full rounded-lg border px-3 py-2"></textarea>
			</div>

			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
				<div>
					<label for="track_url" class="mb-1 block text-sm font-medium">Link do śladu rejsu</label>
					<input id="track_url" type="url" bind:value={form.track_url} placeholder="https://..." class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="voyage_card" class="mb-1 block text-sm font-medium">Karta rejsu</label>
					<input id="voyage_card" type="text" bind:value={form.voyage_card} placeholder="np. u kapitana, w klubie" class="w-full rounded-lg border px-3 py-2" />
				</div>
			</div>

			<div class="flex gap-3">
				<button type="submit" disabled={submit.saving} class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
					{submit.saving ? 'Zapisywanie...' : 'Zapisz zmiany'}
				</button>
				<a href="/voyages/{id}" class="tap-target rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50">Anuluj</a>
			</div>
		</form>
	{/key}
</div>
