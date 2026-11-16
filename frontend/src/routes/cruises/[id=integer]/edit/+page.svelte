<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { updateCruise } from '#lib/api/routes.js';
	import BackLink from '#lib/components/BackLink.svelte';
	import type { Cruise } from '#lib/api/types.js';
	import { goto } from '$app/navigation';
	import { cruiseBody } from '#lib/cruise.js';
	import CruiseSeriesFields from '#lib/components/CruiseSeriesFields.svelte';
	import PriceCoverageFields from '#lib/components/PriceCoverageFields.svelte';
	import { listText, parseList } from '#lib/pricing.js';
	import CruiseLogisticsFields from '#lib/components/CruiseLogisticsFields.svelte';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import CountryPicker from '#lib/location/CountryPicker.svelte';
	import { perLoad } from '#lib/state.svelte.js';

	let { data } = $props();

	const id = $derived(data.cruise.id);
	let form = $derived(perLoad(data, cruiseForm(data.cruise)));
	let included = $derived(listText(data.cruise.included));
	let notIncluded = $derived(listText(data.cruise.not_included));
	let submit = $derived(perLoad(data, { error: '', saving: false }));

	$effect(() => {
		if (auth.user && !auth.isAdmin) {
			void goto(`/cruises/${id}`);
		}
	});

	function cruiseForm(cruise: Cruise) {
		return {
			name: cruise.name,
			series_id: cruise.series_id ?? null,
			edition: cruise.edition ?? '',
			countries: [...(cruise.countries ?? [])],
			description: cruise.description ?? '',
			travel_notes: cruise.travel_notes ?? '',
			min_age: cruise.min_age ?? null
		};
	}

	async function handleSubmit(e: Event) {
		e.preventDefault();
		const run = submit;
		const cruiseId = id;
		run.saving = true;
		run.error = '';
		try {
			await updateCruise(cruiseId, {
				...cruiseBody(form),
				included: parseList(included),
				not_included: parseList(notIncluded)
			});
			if (run === submit) void goto(`/cruises/${cruiseId}`);
		} catch (err) {
			run.error = err instanceof Error ? err.message : 'Nie udało się zapisać';
		} finally {
			run.saving = false;
		}
	}
</script>

<div class="mx-auto max-w-3xl">
	<BackLink href="/cruises/{id}" label={data.cruise.name} />
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Edytuj wydarzenie</h1>
	<LoadErrorBanner errors={data.loadErrors} />

	{#if submit.error}
		<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{submit.error}</div>
	{/if}

	<form onsubmit={handleSubmit} class="space-y-6 rounded-2xl bg-white p-6 shadow-sm">
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
			<div class="sm:col-span-2">
				<label for="name" class="mb-1 block text-sm font-medium">Nazwa wydarzenia *</label>
				<input
					id="name"
					type="text"
					bind:value={form.name}
					required
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
			<CruiseSeriesFields
				bind:seriesId={form.series_id}
				bind:edition={form.edition}
				series={data.series}
			/>
			<div class="sm:col-span-2">
				<label for="countries" class="mb-1 block text-sm font-medium">Kraje</label>
				<CountryPicker id="countries" bind:value={form.countries} countries={data.countries} />
			</div>
			<PriceCoverageFields bind:included bind:notIncluded />
			<CruiseLogisticsFields bind:travelNotes={form.travel_notes} bind:minAge={form.min_age} />
		</div>

		<div>
			<label for="description" class="mb-1 block text-sm font-medium">Opis</label>
			<textarea
				id="description"
				bind:value={form.description}
				rows="4"
				class="w-full rounded-lg border px-3 py-2"></textarea>
		</div>

		<div class="flex gap-3">
			<button
				type="submit"
				disabled={submit.saving}
				class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
			>
				{submit.saving ? 'Zapisywanie...' : 'Zapisz zmiany'}
			</button>
			<a
				href="/cruises/{id}"
				class="tap-target rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50"
				>Anuluj</a
			>
		</div>
	</form>
</div>
