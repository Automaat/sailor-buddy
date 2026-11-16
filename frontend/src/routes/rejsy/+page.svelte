<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { createSeries } from '#lib/api/routes.js';
	import { cruiseKindLabels, type CruiseKind } from '#lib/cruise.js';
	import { formatDateRangeWithDays } from '#lib/dates.js';
	import { formatRoute } from '#lib/location/route.js';
	import { canSeeHidden } from '#lib/publication.js';
	import {
		isStandaloneHidden,
		standaloneHref,
		standaloneStatus,
		standaloneStatusBadges,
		standaloneStatusLabels
	} from '#lib/rejsy.js';
	import SeriesKindSelect from '#lib/components/SeriesKindSelect.svelte';
	import HiddenBadge from '#lib/components/HiddenBadge.svelte';
	import LoadErrorBanner from '#lib/components/LoadErrorBanner.svelte';
	import Sailboat from '@lucide/svelte/icons/sailboat';

	let { data } = $props();

	let series = $derived(data.series);
	const standalone = $derived(data.standalone);
	const showHidden = $derived(canSeeHidden(auth));
	let showForm = $state(false);
	let form = $state({ name: '', description: '', kind: 'sea' as CruiseKind });
	let saving = $state(false);
	let error = $state('');

	async function handleAdd(e: Event) {
		e.preventDefault();
		saving = true;
		error = '';
		try {
			const created = await createSeries({
				name: form.name,
				description: form.description || undefined,
				kind: form.kind
			});
			series = [...series, created].sort((a, b) => a.name.localeCompare(b.name));
			form = { name: '', description: '', kind: 'sea' };
			showForm = false;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się dodać serii';
		} finally {
			saving = false;
		}
	}
</script>

<div class="mx-auto max-w-5xl">
	<div class="mb-6 flex flex-wrap items-start justify-between gap-3">
		<div class="min-w-0">
			<h1 class="text-3xl font-bold text-[var(--navy)]">Rejsy</h1>
			<p class="mt-1 text-sm text-[var(--text-muted)]">Serie rejsów klubu i rejsy bez serii</p>
		</div>
		{#if auth.isAdmin}
			<div class="flex flex-wrap gap-2">
				<a
					href="/trips/new"
					class="tap-target rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--ocean-dark)]"
				>
					+ Nowy rejs
				</a>
				<button
					type="button"
					onclick={() => (showForm = !showForm)}
					class="tap-target rounded-lg border bg-white px-4 py-2 text-sm font-medium text-[var(--navy)] hover:bg-gray-50"
				>
					{showForm ? 'Anuluj' : '+ Dodaj serię'}
				</button>
			</div>
		{/if}
	</div>

	<LoadErrorBanner errors={data.loadErrors} />

	{#if showForm && auth.isAdmin}
		<form onsubmit={handleAdd} class="mb-6 max-w-3xl rounded-2xl bg-white p-6 shadow-sm">
			{#if error}
				<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
			{/if}
			<div class="grid gap-4">
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
			</div>
			<button
				type="submit"
				disabled={saving}
				class="mt-4 rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
			>
				{saving ? 'Dodawanie...' : 'Dodaj'}
			</button>
		</form>
	{/if}

	{#if series.length === 0 && standalone.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Sailboat class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p class="mt-4 text-lg text-[var(--text-muted)]">Brak rejsów</p>
		</div>
	{/if}

	{#if series.length > 0}
		<ul class="grid grid-cols-1 gap-3 lg:grid-cols-2" aria-label="Serie">
			{#each series as s (s.id)}
				<li class="min-w-0">
					<a
						href="/rejsy/{s.id}"
						class="block h-full rounded-2xl bg-white px-6 py-4 shadow-sm transition-shadow hover:shadow-md max-sm:px-4"
					>
						<div class="flex flex-wrap items-center gap-2">
							<span class="min-w-0 font-semibold [overflow-wrap:anywhere] text-[var(--navy)]"
								>{s.name}</span
							>
							<span
								class="shrink-0 rounded-full bg-[var(--ocean)]/10 px-2 py-0.5 text-xs text-[var(--ocean)]"
								>{cruiseKindLabels[s.kind]}</span
							>
						</div>
						{#if s.description}
							<p
								class="mt-1 line-clamp-2 text-sm [overflow-wrap:anywhere] text-[var(--text-muted)]"
							>
								{s.description}
							</p>
						{/if}
					</a>
				</li>
			{/each}
		</ul>
	{/if}

	{#if standalone.length > 0}
		<section aria-labelledby="no-series" class="mt-10">
			<h2 id="no-series" class="mb-1 text-xl font-semibold text-[var(--navy)]">Bez serii</h2>
			<p class="mb-4 text-sm text-[var(--text-muted)]">Pojedyncze rejsy spoza serii</p>
			<ul class="grid grid-cols-1 gap-3 lg:grid-cols-2">
				{#each standalone as entry (`${entry.kind}-${entry.id}`)}
					{@const status = standaloneStatus(entry)}
					{@const route = formatRoute(entry.start_port, entry.end_port)}
					<li class="min-w-0" data-testid="standalone-entry">
						<a
							href={standaloneHref(entry)}
							class="block h-full rounded-2xl bg-white px-6 py-4 shadow-sm transition-shadow hover:shadow-md max-sm:px-4"
						>
							<div class="flex flex-wrap items-center gap-2">
								<span class="min-w-0 font-semibold [overflow-wrap:anywhere] text-[var(--navy)]"
									>{entry.name}</span
								>
								{#if status}
									<span
										class="shrink-0 rounded-full px-2 py-0.5 text-xs font-medium whitespace-nowrap {standaloneStatusBadges[
											status
										]}"
									>
										{standaloneStatusLabels[status]}
									</span>
								{/if}
								{#if showHidden && isStandaloneHidden(entry)}<HiddenBadge />{/if}
							</div>
							{#if entry.kind !== 'cruise' && entry.cruise_name !== entry.name}
								<p class="mt-1 text-sm [overflow-wrap:anywhere] text-[var(--text-muted)]">
									{entry.cruise_name}
								</p>
							{/if}
							<p class="mt-1 text-sm text-[var(--navy)]">
								{#if entry.kind === 'cruise'}
									Brak etapów
								{:else if entry.embark_date || entry.disembark_date}
									{formatDateRangeWithDays(entry.embark_date, entry.disembark_date)}
								{:else}
									Termin do ustalenia
								{/if}
							</p>
							{#if route}
								<p class="mt-1 text-sm [overflow-wrap:anywhere] text-[var(--text-muted)]">
									{route}
								</p>
							{/if}
						</a>
					</li>
				{/each}
			</ul>
		</section>
	{/if}
</div>
