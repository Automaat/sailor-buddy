<script lang="ts">
	import { api } from '$lib/api/client';
	import { goto } from '$app/navigation';
	import { orgStore } from '$lib/stores/org.svelte';
	import type { Cruise } from '$lib/api/types';

	let error = $state('');
	let loading = $state(false);

	$effect(() => {
		if (orgStore.loaded && !orgStore.isOrgAdmin) {
			goto('/cruises');
		}
	});

	let form = $state({
		name: '',
		embark_date: '',
		disembark_date: '',
		countries: '',
		start_port: '',
		end_port: '',
		max_crew: 0,
		cost_per_person: 0,
		description: ''
	});

	async function handleSubmit(e: Event) {
		e.preventDefault();
		if (!orgStore.isOrgMode) {
			error = 'Wydarzenia istnieją tylko w klubie';
			return;
		}
		if (!orgStore.isOrgAdmin) {
			error = 'Tylko administratorzy mogą tworzyć wydarzenia';
			return;
		}
		loading = true;
		error = '';
		try {
			const cruise = await api.post<Cruise>(`${orgStore.apiPrefix()}/cruises`, form);
			goto(`/cruises/${cruise.id}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się utworzyć wydarzenia';
		} finally {
			loading = false;
		}
	}
</script>

<div class="mx-auto max-w-3xl">
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Nowe wydarzenie</h1>

	{#if error}
		<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
	{/if}

	<form onsubmit={handleSubmit} class="space-y-6 rounded-2xl bg-white p-6 shadow-sm">
		<div class="grid grid-cols-2 gap-4">
			<div class="col-span-2">
				<label for="name" class="mb-1 block text-sm font-medium">Nazwa wydarzenia *</label>
				<input id="name" type="text" bind:value={form.name} required class="w-full rounded-lg border px-3 py-2" />
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
			<div class="col-span-2">
				<label for="countries" class="mb-1 block text-sm font-medium">Kraje</label>
				<input id="countries" type="text" bind:value={form.countries} class="w-full rounded-lg border px-3 py-2" />
			</div>
			<div>
				<label for="max_crew" class="mb-1 block text-sm font-medium">Maks. liczba uczestników</label>
				<input id="max_crew" type="number" bind:value={form.max_crew} min="0" class="w-full rounded-lg border px-3 py-2" placeholder="0 = bez limitu" />
			</div>
			<div>
				<label for="cost_pp" class="mb-1 block text-sm font-medium">Szacowany koszt / osobę</label>
				<input id="cost_pp" type="number" step="0.01" bind:value={form.cost_per_person} class="w-full rounded-lg border px-3 py-2" />
			</div>
		</div>

		<div>
			<label for="description" class="mb-1 block text-sm font-medium">Opis</label>
			<textarea id="description" bind:value={form.description} rows="4" class="w-full rounded-lg border px-3 py-2"></textarea>
		</div>

		<div class="flex gap-3">
			<button type="submit" disabled={loading} class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
				{loading ? 'Tworzenie...' : 'Utwórz wydarzenie'}
			</button>
			<a href="/cruises" class="rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50">Anuluj</a>
		</div>
	</form>
</div>
