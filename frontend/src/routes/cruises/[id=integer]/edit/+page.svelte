<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { getCruise, updateCruise } from '#lib/api/routes.js';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { cruiseKinds, cruiseKindLabels, type CruiseKind } from '#lib/cruise.js';
	import CruiseSeriesFields from '#lib/components/CruiseSeriesFields.svelte';
	import PriceFields from '#lib/components/PriceFields.svelte';
	import { listText, parseList, priceBody, priceFormFrom } from '#lib/pricing.js';

	let error = $state('');
	let loading = $state(true);
	let saving = $state(false);

	let form = $state({
		name: '',
		kind: 'sea' as CruiseKind,
		series_id: null as number | null,
		edition: '',
		embark_date: '',
		disembark_date: '',
		countries: '',
		start_port: '',
		end_port: '',
		max_crew: 0,
		description: ''
	});
	let price = $state(priceFormFrom());
	let included = $state('');
	let notIncluded = $state('');

	const id = $derived(Number(page.params.id));

	$effect(() => {
		if (auth.user && !auth.isAdmin) {
			goto(`/cruises/${id}`);
		}
	});

	onMount(async () => {
		try {
			const cruise = await getCruise(id);
			if (!cruise) {
				error = 'Nie znaleziono wydarzenia';
				return;
			}
			form = {
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
				description: cruise.description ?? ''
			};
			price = priceFormFrom(cruise);
			included = listText(cruise.included);
			notIncluded = listText(cruise.not_included);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się wczytać';
		} finally {
			loading = false;
		}
	});

	async function handleSubmit(e: Event) {
		e.preventDefault();
		saving = true;
		error = '';
		try {
			await updateCruise(id, {
				...form,
				...priceBody(price),
				included: parseList(included),
				not_included: parseList(notIncluded),
				series_id: form.series_id ?? undefined
			});
			goto(`/cruises/${id}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się zapisać';
		} finally {
			saving = false;
		}
	}
</script>

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else}
	<div class="mx-auto max-w-3xl">
		<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Edytuj wydarzenie</h1>

		{#if error}
			<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
		{/if}

		<form onsubmit={handleSubmit} class="space-y-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="grid grid-cols-2 gap-4">
				<div class="col-span-2">
					<label for="name" class="mb-1 block text-sm font-medium">Nazwa wydarzenia *</label>
					<input id="name" type="text" bind:value={form.name} required class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div class="col-span-2">
					<label for="kind" class="mb-1 block text-sm font-medium">Rodzaj rejsu</label>
					<select id="kind" bind:value={form.kind} class="w-full rounded-lg border px-3 py-2">
						{#each cruiseKinds as kind (kind)}
							<option value={kind}>{cruiseKindLabels[kind]}</option>
						{/each}
					</select>
				</div>
				<CruiseSeriesFields bind:seriesId={form.series_id} bind:edition={form.edition} />
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
				<div class="col-span-2">
					<label for="countries" class="mb-1 block text-sm font-medium">Kraje</label>
					<input id="countries" type="text" bind:value={form.countries} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="max_crew" class="mb-1 block text-sm font-medium">Maks. liczba uczestników</label>
					<input id="max_crew" type="number" bind:value={form.max_crew} min="0" class="w-full rounded-lg border px-3 py-2" />
				</div>
				<PriceFields bind:price bind:included bind:notIncluded />
			</div>

			<div>
				<label for="description" class="mb-1 block text-sm font-medium">Opis</label>
				<textarea id="description" bind:value={form.description} rows="4" class="w-full rounded-lg border px-3 py-2"></textarea>
			</div>

			<div class="flex gap-3">
				<button type="submit" disabled={saving} class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
					{saving ? 'Zapisywanie...' : 'Zapisz zmiany'}
				</button>
				<a href="/cruises/{id}" class="rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50">Anuluj</a>
			</div>
		</form>
	</div>
{/if}
