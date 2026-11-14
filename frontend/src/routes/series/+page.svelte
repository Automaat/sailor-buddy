<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { listSeries, createSeries } from '#lib/api/routes.js';
	import type { CruiseSeries } from '#lib/api/types.js';
	import Repeat from '@lucide/svelte/icons/repeat';

	let series = $state<CruiseSeries[]>([]);
	let loading = $state(true);
	let showForm = $state(false);
	let form = $state({ name: '', description: '' });
	let saving = $state(false);
	let error = $state('');

	async function load() {
		loading = true;
		try {
			series = await listSeries();
		} catch (err) {
			console.error('Failed to load series:', err);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		void load();
	});

	async function handleAdd(e: Event) {
		e.preventDefault();
		saving = true;
		error = '';
		try {
			const created = await createSeries({ name: form.name, description: form.description || undefined });
			series = [...series, created].sort((a, b) => a.name.localeCompare(b.name));
			form = { name: '', description: '' };
			showForm = false;
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się dodać serii';
		} finally {
			saving = false;
		}
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between max-md:flex-wrap max-md:gap-3">
		<h1 class="text-3xl font-bold text-[var(--navy)]">Serie rejsów</h1>
		{#if auth.isAdmin}
			<button onclick={() => (showForm = !showForm)} class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--ocean-dark)]">
				{showForm ? 'Anuluj' : '+ Dodaj serię'}
			</button>
		{/if}
	</div>

	{#if showForm && auth.isAdmin}
		<form onsubmit={handleAdd} class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			{#if error}
				<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
			{/if}
			<div class="grid gap-4">
				<div>
					<label for="name" class="mb-1 block text-sm font-medium">Nazwa *</label>
					<input id="name" type="text" bind:value={form.name} required class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="description" class="mb-1 block text-sm font-medium">Opis</label>
					<textarea id="description" bind:value={form.description} rows="3" class="w-full rounded-lg border px-3 py-2"></textarea>
				</div>
			</div>
			<button type="submit" disabled={saving} class="mt-4 rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
				{saving ? 'Dodawanie...' : 'Dodaj'}
			</button>
		</form>
	{/if}

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if series.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Repeat class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p class="mt-4 text-lg text-[var(--text-muted)]">Brak serii</p>
		</div>
	{:else}
		<div class="grid gap-3">
			{#each series as s (s.id)}
				<a href="/series/{s.id}" class="block rounded-2xl bg-white px-6 py-4 shadow-sm hover:shadow-md">
					<span class="font-semibold text-[var(--navy)]">{s.name}</span>
					{#if s.description}
						<p class="mt-1 line-clamp-2 text-sm text-[var(--text-muted)]">{s.description}</p>
					{/if}
				</a>
			{/each}
		</div>
	{/if}
</div>
