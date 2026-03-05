<script lang="ts">
	import { api } from '$lib/api/client';
	import type { Cruise, Yacht } from '$lib/api/types';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let error = $state('');
	let loading = $state(true);
	let saving = $state(false);
	let yachts = $state<Yacht[]>([]);

	let form = $state({
		name: '',
		year: 0,
		embark_date: '',
		disembark_date: '',
		countries: '',
		start_port: '',
		end_port: '',
		hours_total: 0,
		hours_sail: 0,
		hours_engine: 0,
		hours_over_6bf: 0,
		miles: 0,
		days: 0,
		captain_name: '',
		yacht_id: 0,
		tidal_waters: false,
		cost_total: 0,
		cost_per_person: 0,
		description: ''
	});

	const id = $derived(page.params.id);

	onMount(async () => {
		try {
			const [cruise, y] = await Promise.all([
				api.get<Cruise>(`/cruises/${id}`),
				api.get<Yacht[]>('/yachts')
			]);
			yachts = y;
			form = {
				name: cruise.name,
				year: cruise.year ?? 0,
				embark_date: cruise.embark_date ?? '',
				disembark_date: cruise.disembark_date ?? '',
				countries: cruise.countries ?? '',
				start_port: cruise.start_port ?? '',
				end_port: cruise.end_port ?? '',
				hours_total: cruise.hours_total ?? 0,
				hours_sail: cruise.hours_sail ?? 0,
				hours_engine: cruise.hours_engine ?? 0,
				hours_over_6bf: cruise.hours_over_6bf ?? 0,
				miles: cruise.miles ?? 0,
				days: cruise.days ?? 0,
				captain_name: cruise.captain_name ?? '',
				yacht_id: cruise.yacht_id ?? 0,
				tidal_waters: !!cruise.tidal_waters,
				cost_total: cruise.cost_total ?? 0,
				cost_per_person: cruise.cost_per_person ?? 0,
				description: cruise.description ?? ''
			};
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to load';
		} finally {
			loading = false;
		}
	});

	async function handleSubmit(e: Event) {
		e.preventDefault();
		saving = true;
		error = '';
		try {
			await api.put(`/cruises/${id}`, form);
			goto(`/cruises/${id}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Failed to update';
		} finally {
			saving = false;
		}
	}
</script>

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Loading...</div>
{:else}
	<div class="mx-auto max-w-3xl">
		<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Edit Cruise</h1>

		{#if error}
			<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
		{/if}

		<form onsubmit={handleSubmit} class="space-y-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="grid grid-cols-2 gap-4">
				<div class="col-span-2">
					<label for="name" class="mb-1 block text-sm font-medium">Cruise Name *</label>
					<input id="name" type="text" bind:value={form.name} required class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="year" class="mb-1 block text-sm font-medium">Year</label>
					<input id="year" type="number" bind:value={form.year} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="yacht" class="mb-1 block text-sm font-medium">Yacht</label>
					<select id="yacht" bind:value={form.yacht_id} class="w-full rounded-lg border px-3 py-2">
						<option value={0}>-- Select --</option>
						{#each yachts as yacht}
							<option value={yacht.id}>{yacht.name}</option>
						{/each}
					</select>
				</div>
				<div>
					<label for="embark" class="mb-1 block text-sm font-medium">Embark Date</label>
					<input id="embark" type="date" bind:value={form.embark_date} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="disembark" class="mb-1 block text-sm font-medium">Disembark Date</label>
					<input id="disembark" type="date" bind:value={form.disembark_date} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="start_port" class="mb-1 block text-sm font-medium">Start Port</label>
					<input id="start_port" type="text" bind:value={form.start_port} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="end_port" class="mb-1 block text-sm font-medium">End Port</label>
					<input id="end_port" type="text" bind:value={form.end_port} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="countries" class="mb-1 block text-sm font-medium">Countries</label>
					<input id="countries" type="text" bind:value={form.countries} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="captain" class="mb-1 block text-sm font-medium">Captain</label>
					<input id="captain" type="text" bind:value={form.captain_name} class="w-full rounded-lg border px-3 py-2" />
				</div>
			</div>

			<hr />
			<h3 class="font-semibold text-[var(--navy)]">Navigation Stats</h3>
			<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
				<div>
					<label for="ht" class="mb-1 block text-sm font-medium">Total Hours</label>
					<input id="ht" type="number" step="0.1" bind:value={form.hours_total} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="hs" class="mb-1 block text-sm font-medium">Sail Hours</label>
					<input id="hs" type="number" step="0.1" bind:value={form.hours_sail} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="he" class="mb-1 block text-sm font-medium">Engine Hours</label>
					<input id="he" type="number" step="0.1" bind:value={form.hours_engine} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="h6" class="mb-1 block text-sm font-medium">Hours &gt;6Bf</label>
					<input id="h6" type="number" step="0.1" bind:value={form.hours_over_6bf} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="mi" class="mb-1 block text-sm font-medium">Miles</label>
					<input id="mi" type="number" step="0.1" bind:value={form.miles} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="da" class="mb-1 block text-sm font-medium">Days</label>
					<input id="da" type="number" bind:value={form.days} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div class="flex items-end">
					<label class="flex items-center gap-2 text-sm">
						<input type="checkbox" bind:checked={form.tidal_waters} />
						Tidal Waters
					</label>
				</div>
			</div>

			<hr />
			<h3 class="font-semibold text-[var(--navy)]">Costs</h3>
			<div class="grid grid-cols-2 gap-4">
				<div>
					<label for="ct" class="mb-1 block text-sm font-medium">Total Cost</label>
					<input id="ct" type="number" step="0.01" bind:value={form.cost_total} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="cp" class="mb-1 block text-sm font-medium">Cost per Person</label>
					<input id="cp" type="number" step="0.01" bind:value={form.cost_per_person} class="w-full rounded-lg border px-3 py-2" />
				</div>
			</div>

			<div>
				<label for="desc" class="mb-1 block text-sm font-medium">Description</label>
				<textarea id="desc" bind:value={form.description} rows="4" class="w-full rounded-lg border px-3 py-2"></textarea>
			</div>

			<div class="flex gap-3">
				<button type="submit" disabled={saving} class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
					{saving ? 'Saving...' : 'Save Changes'}
				</button>
				<a href="/cruises/{id}" class="rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50">Cancel</a>
			</div>
		</form>
	</div>
{/if}
