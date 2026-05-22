<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';
	import { page } from '$app/state';
	import { listCruises, createTrip } from '$lib/api/routes';
	import type { Cruise } from '$lib/api/types';
	import { onMount } from 'svelte';

	let cruises = $state<Cruise[]>([]);
	let error = $state('');
	let loading = $state(false);

	// Planning trips is an admin task; redirect regular members away.
	$effect(() => {
		if (auth.user && !auth.isAdmin) {
			goto('/trips');
		}
	});

	const initialCruiseID = $derived.by(() => {
		const v = page.url.searchParams.get('cruise_id');
		const n = v ? Number(v) : 0;
		return Number.isFinite(n) ? n : 0;
	});

	let form = $state({
		name: '',
		embark_date: '',
		disembark_date: '',
		countries: '',
		start_port: '',
		end_port: '',
		cruise_id: 0,
		cost_per_person: 0,
		max_crew: 0,
		description: ''
	});

	onMount(async () => {
		cruises = await listCruises().catch(() => []);
		if (initialCruiseID) {
			form.cruise_id = initialCruiseID;
			const c = cruises.find((x) => x.id === initialCruiseID);
			if (c) {
				if (!form.embark_date) form.embark_date = c.embark_date ?? '';
				if (!form.disembark_date) form.disembark_date = c.disembark_date ?? '';
				if (!form.countries) form.countries = c.countries ?? '';
				if (!form.start_port) form.start_port = c.start_port ?? '';
				if (!form.end_port) form.end_port = c.end_port ?? '';
			}
		}
	});

	async function handleSubmit(e: Event) {
		e.preventDefault();
		loading = true;
		error = '';
		try {
			const payload = {
				...form,
				// The planning user is the captain by default; fall back to the
				// Firebase display name if the DB user has not loaded yet.
				captain_name: auth.user?.name ?? auth.firebaseUser?.displayName ?? undefined,
				cruise_id: form.cruise_id || undefined
			};
			const trip = await createTrip(payload);
			goto(`/trips/${trip.id}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się zaplanować rejsu';
		} finally {
			loading = false;
		}
	}
</script>

<div class="mx-auto max-w-3xl">
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Zaplanuj rejs</h1>

	{#if error}
		<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
	{/if}

	<form onsubmit={handleSubmit} class="space-y-6 rounded-2xl bg-white p-6 shadow-sm">
		<div class="grid grid-cols-2 gap-4">
			<div class="col-span-2">
				<label for="name" class="mb-1 block text-sm font-medium">Nazwa rejsu *</label>
				<input id="name" type="text" bind:value={form.name} required class="w-full rounded-lg border px-3 py-2" />
			</div>
			{#if cruises.length > 0}
				<div class="col-span-2">
					<label for="cruise" class="mb-1 block text-sm font-medium">Wydarzenie klubu</label>
					<select id="cruise" bind:value={form.cruise_id} class="w-full rounded-lg border px-3 py-2">
						<option value={0}>— samodzielny rejs —</option>
						{#each cruises as cruise}
							<option value={cruise.id}>{cruise.name}</option>
						{/each}
					</select>
				</div>
			{/if}
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
		</div>

		<hr />
		<h3 class="font-semibold text-[var(--navy)]">Koszty i załoga</h3>
		<div class="grid grid-cols-2 gap-4">
			<div>
				<label for="cost_pp" class="mb-1 block text-sm font-medium">Koszt na osobę</label>
				<input id="cost_pp" type="number" step="0.01" bind:value={form.cost_per_person} class="w-full rounded-lg border px-3 py-2" />
			</div>
			<div>
				<label for="max_crew" class="mb-1 block text-sm font-medium">Maks. załoga</label>
				<input id="max_crew" type="number" bind:value={form.max_crew} class="w-full rounded-lg border px-3 py-2" min="0" placeholder="0 = bez limitu" />
			</div>
		</div>

		<div>
			<label for="description" class="mb-1 block text-sm font-medium">Opis</label>
			<textarea id="description" bind:value={form.description} rows="4" class="w-full rounded-lg border px-3 py-2"></textarea>
		</div>

		<div class="flex gap-3">
			<button type="submit" disabled={loading} class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
				{loading ? 'Tworzenie...' : 'Zaplanuj rejs'}
			</button>
			<a href="/trips" class="rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50">Anuluj</a>
		</div>
	</form>
</div>
