<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { updateSeries, deleteSeries } from '#lib/api/routes.js';
	import { cruiseKindLabels, type CruiseKind } from '#lib/cruise.js';
	import SeriesKindSelect from '#lib/components/SeriesKindSelect.svelte';
	import { goto } from '$app/navigation';
	import { errorMessage } from '#lib/api/errors.js';
	import HiddenBadge from '#lib/components/HiddenBadge.svelte';
	import { canSeeHidden, isHidden } from '#lib/publication.js';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';

	// data comes from +page.ts, which reruns when the URL's series changes.
	let { data } = $props();

	// series changes in place after a rename and resets to the loaded value
	// on navigation.
	let series = $derived(data.series);
	const editions = $derived(data.editions);
	const id = $derived(data.series.id);
	const showHidden = $derived(canSeeHidden(auth));
	let editing = $state(false);
	let saving = $state(false);
	let error = $state('');
	let deleteError = $state('');
	let form = $state({ name: '', description: '', kind: 'sea' as CruiseKind });

	// The edit form, its error and busy state belong to one series; a link to
	// another series closes them.
	$effect(() => {
		void id;
		editing = false;
		saving = false;
		error = '';
		deleteError = '';
	});

	function startEdit() {
		form = { name: series.name, description: series.description ?? '', kind: series.kind };
		error = '';
		deleteError = '';
		editing = true;
	}

	async function handleSave(e: Event) {
		e.preventDefault();
		const seriesId = id;
		const { name, kind } = form;
		const description = form.description || undefined;
		saving = true;
		error = '';
		try {
			await updateSeries(seriesId, { name, description, kind });
			// A reply for a series the page has left is dropped.
			if (seriesId !== id) return;
			series = { ...series, name, description, kind };
			editing = false;
		} catch (err) {
			if (seriesId !== id) return;
			error = err instanceof Error ? err.message : 'Nie udało się zapisać';
		} finally {
			if (seriesId === id) saving = false;
		}
	}

	async function handleDelete() {
		if (!confirm('Usunąć serię? Wydarzenia zostaną, ale stracą powiązanie z serią.')) return;
		const seriesId = id;
		deleteError = '';
		try {
			await deleteSeries(seriesId);
		} catch (err) {
			console.error('Failed to delete series:', err);
			if (seriesId === id) deleteError = `Nie udało się usunąć serii: ${errorMessage(err)}`;
			return;
		}
		if (seriesId !== id) return;
		void goto('/series');
	}
</script>

<div class="mx-auto max-w-5xl">
	{#if editing}
		<form onsubmit={handleSave} class="mb-6 space-y-4 rounded-2xl bg-white p-6 shadow-sm">
			{#if error}
				<div class="rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
			{/if}
			<div>
				<label for="name" class="mb-1 block text-sm font-medium">Nazwa *</label>
				<input
					id="name"
					type="text"
					bind:value={form.name}
					required
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
			<SeriesKindSelect bind:kind={form.kind} />
			<div>
				<label for="description" class="mb-1 block text-sm font-medium">Opis</label>
				<textarea
					id="description"
					bind:value={form.description}
					rows="3"
					class="w-full rounded-lg border px-3 py-2"></textarea>
			</div>
			<div class="flex gap-3">
				<button
					type="submit"
					disabled={saving}
					class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
				>
					{saving ? 'Zapisywanie...' : 'Zapisz zmiany'}
				</button>
				<button
					type="button"
					onclick={() => (editing = false)}
					class="rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50"
					>Anuluj</button
				>
			</div>
		</form>
	{:else}
		<div class="mb-6 flex items-start justify-between max-md:flex-wrap max-md:gap-3">
			<div>
				<h1 class="text-3xl font-bold text-[var(--navy)]">{series.name}</h1>
				<p class="mt-1 text-sm text-[var(--text-muted)]">{cruiseKindLabels[series.kind]}</p>
				{#if series.description}
					<p class="mt-2 whitespace-pre-wrap text-[var(--text-muted)]">{series.description}</p>
				{/if}
			</div>
			{#if auth.isAdmin}
				<div class="flex gap-2">
					<button onclick={startEdit} class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50"
						>Edytuj</button
					>
					<button
						onclick={handleDelete}
						class="rounded-lg border border-red-200 px-4 py-2 text-sm text-red-600 hover:bg-red-50"
					>
						Usuń
					</button>
				</div>
			{/if}
		</div>
	{/if}

	{#if deleteError}
		<div role="alert" class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{deleteError}</div>
	{/if}

	<div class="rounded-2xl bg-white p-6 shadow-sm">
		<div class="mb-3 flex items-center justify-between gap-3">
			<h2 class="font-semibold text-[var(--navy)]">Edycje ({editions.length})</h2>
			{#if auth.isAdmin}
				<a
					href="/cruises/new?series_id={id}"
					class="tap-target rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--ocean-dark)]"
				>
					+ Nowa edycja
				</a>
			{/if}
		</div>
		{#if editions.length === 0}
			<p class="text-sm text-[var(--text-muted)]">
				{auth.isAdmin ? 'Brak edycji. Dodaj pierwszą przyciskiem „+ Nowa edycja”.' : 'Brak edycji.'}
			</p>
		{:else}
			<table class="w-full text-left text-sm">
				<thead>
					<tr class="border-b text-xs font-medium uppercase tracking-wide text-[var(--text-muted)]">
						<th scope="col" class="w-28 pb-2 pr-4 font-medium">Edycja</th>
						<th scope="col" class="pb-2 font-medium">Nazwa</th>
					</tr>
				</thead>
				<tbody class="divide-y">
					{#each editions as e (e.cruise_id)}
						<tr class="group relative hover:bg-[var(--ocean-tint)]/50" data-testid="edition-row">
							<td class="py-3 pr-4 align-middle">
								<span
									class="inline-block min-w-16 rounded-md bg-[var(--ocean-tint)] px-2 py-1 text-center font-semibold tabular-nums text-[var(--ocean)]"
									>{e.edition ?? '—'}</span
								>
							</td>
							<td class="py-3 align-middle">
								<div class="flex items-center gap-2">
									<a
										href="/cruises/{e.cruise_id}"
										class="min-w-0 flex-1 font-medium text-[var(--navy)] after:absolute after:inset-0 group-hover:text-[var(--ocean)]"
										>{e.name}</a
									>
									{#if showHidden && isHidden(e)}<span class="relative"><HiddenBadge /></span>{/if}
									<ChevronRight
										class="h-4 w-4 shrink-0 text-[var(--text-muted)] group-hover:text-[var(--ocean)]"
										aria-hidden="true"
									/>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</div>
</div>
