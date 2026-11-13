<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { getSeries, updateSeries, deleteSeries, listSeriesEditions } from '#lib/api/routes.js';
	import type { CruiseSeries, CruiseSeriesEdition } from '#lib/api/types.js';
	import { ApiError } from '#lib/api/client.js';
	import NotFound from '#lib/components/NotFound.svelte';
	import { cruiseKindLabels } from '#lib/cruise.js';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let series = $state<CruiseSeries | null>(null);
	let editions = $state<CruiseSeriesEdition[]>([]);
	let loading = $state(true);
	let notFound = $state(false);
	let editing = $state(false);
	let saving = $state(false);
	let error = $state('');
	let form = $state({ name: '', description: '' });

	const id = $derived(Number(page.params.id));
	const totalMiles = $derived(editions.reduce((sum, e) => sum + e.total_miles, 0));
	const totalHours = $derived(editions.reduce((sum, e) => sum + e.total_hours, 0));

	function formatNumber(n: number) {
		return n.toLocaleString('pl-PL', { maximumFractionDigits: 1 });
	}

	onMount(async () => {
		try {
			const [s, e] = await Promise.all([getSeries(id), listSeriesEditions(id)]);
			series = s;
			editions = e ?? [];
		} catch (err) {
			if (err instanceof ApiError && err.status === 404) {
				notFound = true;
			} else {
				console.error('Failed to load series:', err);
			}
		} finally {
			loading = false;
		}
	});

	function startEdit() {
		if (!series) return;
		form = { name: series.name, description: series.description ?? '' };
		error = '';
		editing = true;
	}

	async function handleSave(e: Event) {
		e.preventDefault();
		if (!series) return;
		saving = true;
		error = '';
		try {
			const description = form.description || undefined;
			await updateSeries(id, { name: form.name, description });
			series = { ...series, name: form.name, description };
			editing = false;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się zapisać';
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!confirm('Usunąć serię? Wydarzenia zostaną, ale stracą powiązanie z serią.')) return;
		await deleteSeries(id);
		goto('/series');
	}
</script>

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else if series}
	<div class="mx-auto max-w-5xl">
		{#if editing}
			<form onsubmit={handleSave} class="mb-6 space-y-4 rounded-2xl bg-white p-6 shadow-sm">
				{#if error}
					<div class="rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
				{/if}
				<div>
					<label for="name" class="mb-1 block text-sm font-medium">Nazwa *</label>
					<input id="name" type="text" bind:value={form.name} required class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="description" class="mb-1 block text-sm font-medium">Opis</label>
					<textarea id="description" bind:value={form.description} rows="3" class="w-full rounded-lg border px-3 py-2"></textarea>
				</div>
				<div class="flex gap-3">
					<button type="submit" disabled={saving} class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
						{saving ? 'Zapisywanie...' : 'Zapisz zmiany'}
					</button>
					<button type="button" onclick={() => (editing = false)} class="rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50">Anuluj</button>
				</div>
			</form>
		{:else}
			<div class="mb-6 flex items-start justify-between max-md:flex-wrap max-md:gap-3">
				<div>
					<h1 class="text-3xl font-bold text-[var(--navy)]">{series.name}</h1>
					{#if series.description}
						<p class="mt-2 whitespace-pre-wrap text-[var(--text-muted)]">{series.description}</p>
					{/if}
				</div>
				{#if auth.isAdmin}
					<div class="flex gap-2">
						<button onclick={startEdit} class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">Edytuj</button>
						<button onclick={handleDelete} class="rounded-lg border border-red-200 px-4 py-2 text-sm text-red-600 hover:bg-red-50">
							Usuń
						</button>
					</div>
				{/if}
			</div>
		{/if}

		<div class="rounded-2xl bg-white p-6 shadow-sm">
			<h2 class="mb-3 font-semibold text-[var(--navy)]">Edycje ({editions.length})</h2>
			{#if editions.length === 0}
				<p class="text-sm text-[var(--text-muted)]">Brak edycji. Przypisz wydarzenie do serii w jego edycji.</p>
			{:else}
				<div class="overflow-x-auto">
					<table class="w-full min-w-[32rem] text-left text-sm">
						<thead class="text-[var(--text-muted)]">
							<tr>
								<th class="py-2">Edycja</th>
								<th class="py-2">Wydarzenie</th>
								<th class="py-2">Rodzaj</th>
								<th class="py-2">Termin</th>
								<th class="py-2 text-right">Mile</th>
								<th class="py-2 text-right">Godziny</th>
							</tr>
						</thead>
						<tbody>
							{#each editions as e (e.cruise_id)}
								<tr class="border-t" data-testid="edition-row">
									<td class="py-2 font-medium">{e.edition ?? '—'}</td>
									<td class="py-2"><a href="/cruises/{e.cruise_id}" class="text-[var(--ocean)] hover:underline">{e.name}</a></td>
									<td class="py-2">{cruiseKindLabels[e.kind]}</td>
									<td class="py-2">
										{#if e.embark_date}
											{e.embark_date}{#if e.disembark_date}&nbsp;– {e.disembark_date}{/if}
										{/if}
									</td>
									<td class="py-2 text-right">{formatNumber(e.total_miles)}</td>
									<td class="py-2 text-right">{formatNumber(e.total_hours)}</td>
								</tr>
							{/each}
						</tbody>
						<tfoot>
							<tr class="border-t font-semibold" data-testid="series-totals">
								<td class="py-2" colspan="4">Razem</td>
								<td class="py-2 text-right">{formatNumber(totalMiles)}</td>
								<td class="py-2 text-right">{formatNumber(totalHours)}</td>
							</tr>
						</tfoot>
					</table>
				</div>
			{/if}
		</div>
	</div>
{:else if notFound}
	<NotFound
		title="Nie znaleziono serii"
		message="Ta seria nie istnieje lub nie masz do niej dostępu."
		backHref="/series"
		backLabel="Wszystkie serie"
	/>
{:else}
	<NotFound
		title="Nie udało się wczytać serii"
		message="Coś poszło nie tak. Spróbuj ponownie za chwilę."
		backHref="/series"
		backLabel="Wszystkie serie"
	/>
{/if}
