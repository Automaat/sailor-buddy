<script lang="ts">
	import { api } from '$lib/api/client';
	import { orgStore } from '$lib/stores/org.svelte';
	import type { Trip, Yacht } from '$lib/api/types';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let error = $state('');
	let loading = $state(true);
	let saving = $state(false);
	let yachts = $state<Yacht[]>([]);

	let form = $state({
		name: '',
		embark_date: '',
		disembark_date: '',
		countries: '',
		start_port: '',
		end_port: '',
		captain_name: '',
		yacht_id: 0,
		cost_total: 0,
		cost_per_person: 0,
		max_crew: 0,
		description: ''
	});

	const id = $derived(page.params.id);

	onMount(async () => {
		try {
			const prefix = orgStore.apiPrefix();
			const [trip, y] = await Promise.all([
				api.get<Trip>(`${prefix}/trips/${id}`),
				api.list<Yacht>(`${prefix}/yachts`)
			]);
			yachts = y;
			form = {
				name: trip.name,
				embark_date: trip.embark_date ?? '',
				disembark_date: trip.disembark_date ?? '',
				countries: trip.countries ?? '',
				start_port: trip.start_port ?? '',
				end_port: trip.end_port ?? '',
				captain_name: trip.captain_name ?? '',
				yacht_id: trip.yacht_id ?? 0,
				cost_total: trip.cost_total ?? 0,
				cost_per_person: trip.cost_per_person ?? 0,
				max_crew: trip.max_crew ?? 0,
				description: trip.description ?? ''
			};
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
			const payload = { ...form, yacht_id: form.yacht_id || undefined };
			await api.put(`${orgStore.apiPrefix()}/trips/${id}`, payload);
			goto(`/trips/${id}`);
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
		<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Edytuj rejs</h1>

		{#if error}
			<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
		{/if}

		<form onsubmit={handleSubmit} class="space-y-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="grid grid-cols-2 gap-4">
				<div class="col-span-2">
					<label for="name" class="mb-1 block text-sm font-medium">Nazwa rejsu *</label>
					<input id="name" type="text" bind:value={form.name} required class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="yacht" class="mb-1 block text-sm font-medium">Jacht</label>
					<select id="yacht" bind:value={form.yacht_id} class="w-full rounded-lg border px-3 py-2">
						<option value={0}>-- Wybierz --</option>
						{#each yachts as yacht}
							<option value={yacht.id}>{yacht.name}</option>
						{/each}
					</select>
				</div>
				<div>
					<label for="captain" class="mb-1 block text-sm font-medium">Kapitan</label>
					<input id="captain" type="text" bind:value={form.captain_name} class="w-full rounded-lg border px-3 py-2" />
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
			</div>

			<hr />
			<h3 class="font-semibold text-[var(--navy)]">Koszty i załoga</h3>
			<div class="grid grid-cols-3 gap-4">
				<div>
					<label for="ct" class="mb-1 block text-sm font-medium">Koszt całkowity</label>
					<input id="ct" type="number" step="0.01" bind:value={form.cost_total} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="cp" class="mb-1 block text-sm font-medium">Koszt na osobę</label>
					<input id="cp" type="number" step="0.01" bind:value={form.cost_per_person} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="max_crew" class="mb-1 block text-sm font-medium">Maks. załoga</label>
					<input id="max_crew" type="number" bind:value={form.max_crew} class="w-full rounded-lg border px-3 py-2" min="0" placeholder="0 = bez limitu" />
				</div>
			</div>

			<div>
				<label for="desc" class="mb-1 block text-sm font-medium">Opis</label>
				<textarea id="desc" bind:value={form.description} rows="4" class="w-full rounded-lg border px-3 py-2"></textarea>
			</div>

			<div class="flex gap-3">
				<button type="submit" disabled={saving} class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
					{saving ? 'Zapisywanie...' : 'Zapisz zmiany'}
				</button>
				<a href="/trips/{id}" class="rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50">Anuluj</a>
			</div>
		</form>
	</div>
{/if}
