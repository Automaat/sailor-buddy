<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { updateCruise } from '#lib/api/routes.js';
	import type { Cruise } from '#lib/api/types.js';
	import { goto } from '$app/navigation';
	import { cruiseBody, cruiseKinds, cruiseKindLabels } from '#lib/cruise.js';
	import CruiseSeriesFields from '#lib/components/CruiseSeriesFields.svelte';
	import PriceFields from '#lib/components/PriceFields.svelte';
	import { listText, parseList, priceBody, priceFormFrom } from '#lib/pricing.js';
	import CruiseLogisticsFields from '#lib/components/CruiseLogisticsFields.svelte';
	import { perLoad } from '#lib/state.svelte.js';

	let { data } = $props();

	const id = $derived(data.cruise.id);
	let form = $derived(perLoad(data, cruiseForm(data.cruise)));
	let price = $derived(perLoad(data, priceFormFrom(data.cruise)));
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
			kind: cruise.kind,
			series_id: cruise.series_id ?? null,
			edition: cruise.edition ?? '',
			embark_date: cruise.embark_date ?? '',
			disembark_date: cruise.disembark_date ?? '',
			countries: cruise.countries ?? '',
			start_port: cruise.start_port ?? '',
			end_port: cruise.end_port ?? '',
			max_crew: cruise.max_crew ?? 0,
			description: cruise.description ?? '',
			group_chat_url: cruise.group_chat_url ?? '',
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
				...priceBody(price),
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
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Edytuj wydarzenie</h1>

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
			<div class="sm:col-span-2">
				<label for="kind" class="mb-1 block text-sm font-medium">Rodzaj rejsu</label>
				<select id="kind" bind:value={form.kind} class="w-full rounded-lg border px-3 py-2">
					{#each cruiseKinds as kind (kind)}
						<option value={kind}>{cruiseKindLabels[kind]}</option>
					{/each}
				</select>
			</div>
			<CruiseSeriesFields
				bind:seriesId={form.series_id}
				bind:edition={form.edition}
				series={data.series}
			/>
			<div>
				<label for="embark" class="mb-1 block text-sm font-medium">Data zaokrętowania</label>
				<input
					id="embark"
					type="date"
					bind:value={form.embark_date}
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
			<div>
				<label for="disembark" class="mb-1 block text-sm font-medium">Data wyokrętowania</label>
				<input
					id="disembark"
					type="date"
					bind:value={form.disembark_date}
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
			<div>
				<label for="start_port" class="mb-1 block text-sm font-medium">Port wyjścia</label>
				<input
					id="start_port"
					type="text"
					bind:value={form.start_port}
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
			<div>
				<label for="end_port" class="mb-1 block text-sm font-medium">Port docelowy</label>
				<input
					id="end_port"
					type="text"
					bind:value={form.end_port}
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
			<div class="sm:col-span-2">
				<label for="countries" class="mb-1 block text-sm font-medium">Kraje</label>
				<input
					id="countries"
					type="text"
					bind:value={form.countries}
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
			<div>
				<label for="max_crew" class="mb-1 block text-sm font-medium">Maks. liczba uczestników</label
				>
				<input
					id="max_crew"
					type="number"
					bind:value={form.max_crew}
					min="0"
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
			<PriceFields bind:price bind:included bind:notIncluded />
			<CruiseLogisticsFields
				bind:groupChatUrl={form.group_chat_url}
				bind:travelNotes={form.travel_notes}
				bind:minAge={form.min_age}
			/>
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
