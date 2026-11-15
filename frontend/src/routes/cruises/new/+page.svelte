<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '#lib/stores/auth.svelte.js';
	import { createCruise } from '#lib/api/routes.js';
	import { cruiseBody, cruiseKinds, cruiseKindLabels, type CruiseKind } from '#lib/cruise.js';
	import PriceFields from '#lib/components/PriceFields.svelte';
	import { parseList, priceBody, priceFormFrom } from '#lib/pricing.js';
	import CruiseLogisticsFields from '#lib/components/CruiseLogisticsFields.svelte';
	import { perLoad } from '#lib/state.svelte.js';

	let { data } = $props();

	const series = $derived(data.series);
	const seriesHref = $derived(`/series/${data.series.id}`);

	$effect(() => {
		if (auth.user && !auth.isAdmin) {
			void goto(seriesHref);
		}
	});

	/** A fresh edition form; a link to another series starts it over. */
	function newDraft() {
		return {
			form: {
				name: '',
				kind: 'sea' as CruiseKind,
				edition: '',
				embark_date: '',
				disembark_date: '',
				countries: '',
				start_port: '',
				end_port: '',
				max_crew: 0,
				description: '',
				group_chat_url: '',
				travel_notes: '',
				min_age: null as number | null
			},
			price: priceFormFrom(),
			included: '',
			notIncluded: '',
			error: '',
			saving: false
		};
	}

	let draft = $derived(perLoad(data, newDraft()));
	const form = $derived(draft.form);

	async function handleSubmit(e: Event) {
		e.preventDefault();
		const run = draft;
		const seriesId = data.series.id;
		if (!auth.isAdmin) {
			run.error = 'Tylko administratorzy mogą tworzyć edycje';
			return;
		}
		if (!run.form.edition.trim()) {
			run.error = 'Podaj oznaczenie edycji';
			return;
		}
		run.saving = true;
		run.error = '';
		try {
			await createCruise({
				...cruiseBody({ ...run.form, series_id: seriesId }),
				...priceBody(run.price),
				included: parseList(run.included),
				not_included: parseList(run.notIncluded)
			});
			if (run === draft) void goto(`/series/${seriesId}`);
		} catch (err) {
			run.error = err instanceof Error ? err.message : 'Nie udało się utworzyć edycji';
		} finally {
			run.saving = false;
		}
	}
</script>

<div class="mx-auto max-w-3xl">
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Nowa edycja</h1>

	{#if draft.error}
		<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{draft.error}</div>
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
			<div>
				<label for="series" class="mb-1 block text-sm font-medium">Seria</label>
				<input
					id="series"
					type="text"
					value={series.name}
					readonly
					class="w-full rounded-lg border bg-gray-50 px-3 py-2 text-[var(--text-muted)]"
				/>
			</div>
			<div>
				<label for="edition" class="mb-1 block text-sm font-medium">Edycja *</label>
				<input
					id="edition"
					type="text"
					bind:value={form.edition}
					required
					maxlength="50"
					placeholder="np. 2025 lub 2026/27"
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
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
					placeholder="0 = bez limitu"
				/>
			</div>
			<PriceFields
				bind:price={draft.price}
				bind:included={draft.included}
				bind:notIncluded={draft.notIncluded}
			/>
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
				disabled={draft.saving}
				class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
			>
				{draft.saving ? 'Tworzenie...' : 'Utwórz edycję'}
			</button>
			<a
				href={seriesHref}
				class="tap-target rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50"
				>Anuluj</a
			>
		</div>
	</form>
</div>
