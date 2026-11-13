<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '#lib/stores/auth.svelte.js';
	import { listYachts, listCruises, createVoyage } from '#lib/api/routes.js';
	import type { Cruise, Yacht } from '#lib/api/types.js';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import CaptainSelect from '#lib/components/CaptainSelect.svelte';
	import CruiseSelect from '#lib/components/CruiseSelect.svelte';
	import YachtSelect from '#lib/components/YachtSelect.svelte';

	let yachts = $state<Yacht[]>([]);
	let cruises = $state<Cruise[]>([]);
	let error = $state('');
	let loading = $state(false);

	// Logging voyages is an admin task; redirect regular members away.
	$effect(() => {
		if (auth.user && !auth.isAdmin) {
			goto('/cruises?tab=completed');
		}
	});

	let form = $state({
		name: '',
		year: new Date().getFullYear(),
		embark_date: '',
		disembark_date: '',
		countries: '',
		start_port: '',
		end_port: '',
		cruise_id: 0,
		hours_total: 0,
		hours_sail: 0,
		hours_engine: 0,
		hours_over_6bf: 0,
		miles: 0,
		days: 0,
		captain_user_id: 0,
		yacht_id: 0,
		tidal_waters: false,
		cost_total: 0,
		cost_per_person: 0,
		description: '',
		track_url: '',
		voyage_card: ''
	});

	onMount(async () => {
		const [y, c] = await Promise.all([listYachts(), listCruises().catch(() => [])]);
		yachts = y;
		cruises = c ?? [];
		const fromQuery = Number(page.url.searchParams.get('cruise_id') ?? 0);
		if (Number.isInteger(fromQuery) && fromQuery > 0) form.cruise_id = fromQuery;
	});

	const cancelHref = $derived(form.cruise_id ? `/cruises/${form.cruise_id}` : '/cruises?tab=completed');

	async function handleSubmit(e: Event) {
		e.preventDefault();
		if (!form.cruise_id) {
			error = 'Wybierz wydarzenie, do którego należy rejs';
			return;
		}
		loading = true;
		error = '';
		try {
			const payload = {
				...form,
				tidal_waters: form.tidal_waters ? 1 : 0,
				yacht_id: form.yacht_id || undefined,
				track_url: form.track_url.trim() || undefined,
				voyage_card: form.voyage_card.trim() || undefined,
				captain_user_id: form.captain_user_id || undefined
			};
			const voyage = await createVoyage(payload);
			goto(`/voyages/${voyage.id}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się dodać rejsu';
		} finally {
			loading = false;
		}
	}
</script>

<div class="mx-auto max-w-3xl">
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Wpisz zrealizowany rejs</h1>

	{#if error}
		<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
	{/if}

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
				<CaptainSelect bind:value={form.captain_user_id} />
			</div>
		</div>

		<hr />
		<h3 class="font-semibold text-[var(--navy)]">Statystyki nawigacyjne</h3>
		<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
			<div>
				<label for="hours_total" class="mb-1 block text-sm font-medium">Godziny łącznie</label>
				<input id="hours_total" type="number" step="0.1" bind:value={form.hours_total} class="w-full rounded-lg border px-3 py-2" />
			</div>
			<div>
				<label for="hours_sail" class="mb-1 block text-sm font-medium">Godziny żagli</label>
				<input id="hours_sail" type="number" step="0.1" bind:value={form.hours_sail} class="w-full rounded-lg border px-3 py-2" />
			</div>
			<div>
				<label for="hours_engine" class="mb-1 block text-sm font-medium">Godziny silnika</label>
				<input id="hours_engine" type="number" step="0.1" bind:value={form.hours_engine} class="w-full rounded-lg border px-3 py-2" />
			</div>
			<div>
				<label for="hours_6bf" class="mb-1 block text-sm font-medium">Godziny &gt;6Bf</label>
				<input id="hours_6bf" type="number" step="0.1" bind:value={form.hours_over_6bf} class="w-full rounded-lg border px-3 py-2" />
			</div>
			<div>
				<label for="miles" class="mb-1 block text-sm font-medium">Mile</label>
				<input id="miles" type="number" step="0.1" bind:value={form.miles} class="w-full rounded-lg border px-3 py-2" />
			</div>
			<div>
				<label for="days" class="mb-1 block text-sm font-medium">Dni</label>
				<input id="days" type="number" bind:value={form.days} class="w-full rounded-lg border px-3 py-2" />
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
				<label for="cost_total" class="mb-1 block text-sm font-medium">Koszt całkowity</label>
				<input id="cost_total" type="number" step="0.01" bind:value={form.cost_total} class="w-full rounded-lg border px-3 py-2" />
			</div>
			<div>
				<label for="cost_pp" class="mb-1 block text-sm font-medium">Koszt na osobę</label>
				<input id="cost_pp" type="number" step="0.01" bind:value={form.cost_per_person} class="w-full rounded-lg border px-3 py-2" />
			</div>
		</div>

		<div>
			<label for="description" class="mb-1 block text-sm font-medium">Opis</label>
			<textarea id="description" bind:value={form.description} rows="4" class="w-full rounded-lg border px-3 py-2"></textarea>
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
			<button type="submit" disabled={loading} class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
				{loading ? 'Tworzenie...' : 'Dodaj rejs'}
			</button>
			<a href={cancelHref} class="tap-target rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50">Anuluj</a>
		</div>
	</form>
</div>
