<script lang="ts">
	import { api } from '$lib/api/client';
	import { goto } from '$app/navigation';
	import { orgStore } from '$lib/stores/org.svelte';
	import type { Yacht } from '$lib/api/types';
	import { onMount } from 'svelte';

	let yachts = $state<Yacht[]>([]);
	let error = $state('');
	let loading = $state(false);

	let form = $state({
		name: '',
		status: 'completed' as 'planned' | 'completed',
		year: new Date().getFullYear(),
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
		max_crew: 0,
		description: ''
	});

	onMount(async () => {
		yachts = await api.get<Yacht[]>(`${orgStore.apiPrefix()}/yachts`);
	});

	async function handleSubmit(e: Event) {
		e.preventDefault();
		loading = true;
		error = '';
		try {
			const payload = { ...form, tidal_waters: form.tidal_waters ? 1 : 0 };
			const cruise = await api.post<{ id: number }>(`${orgStore.apiPrefix()}/cruises`, payload);
			goto(`/cruises/${cruise.id}`);
		} catch (err) {
			error = err instanceof Error ? err.message : 'Nie udało się utworzyć rejsu';
		} finally {
			loading = false;
		}
	}
</script>

<div class="mx-auto max-w-3xl">
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Nowy rejs</h1>

	{#if error}
		<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
	{/if}

	<form onsubmit={handleSubmit} class="space-y-6 rounded-2xl bg-white p-6 shadow-sm">
		<div class="mb-4 flex gap-4">
			<label class="flex items-center gap-2 text-sm font-medium">
				<input type="radio" bind:group={form.status} value="completed" />
				Zrealizowany
			</label>
			<label class="flex items-center gap-2 text-sm font-medium">
				<input type="radio" bind:group={form.status} value="planned" />
				Planowany
			</label>
		</div>

		<div class="grid grid-cols-2 gap-4">
			<div class="col-span-2">
				<label for="name" class="mb-1 block text-sm font-medium">Nazwa rejsu *</label>
				<input
					id="name"
					type="text"
					bind:value={form.name}
					required
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
			<div>
				<label for="year" class="mb-1 block text-sm font-medium">Rok</label>
				<input
					id="year"
					type="number"
					bind:value={form.year}
					class="w-full rounded-lg border px-3 py-2"
				/>
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
			<div>
				<label for="countries" class="mb-1 block text-sm font-medium">Kraje</label>
				<input
					id="countries"
					type="text"
					bind:value={form.countries}
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
			<div>
				<label for="captain" class="mb-1 block text-sm font-medium">Kapitan</label>
				<input
					id="captain"
					type="text"
					bind:value={form.captain_name}
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
		</div>

		{#if form.status === 'completed'}
			<hr />
			<h3 class="font-semibold text-[var(--navy)]">Statystyki nawigacyjne</h3>
			<div class="grid grid-cols-2 gap-4 md:grid-cols-4">
				<div>
					<label for="hours_total" class="mb-1 block text-sm font-medium">Godziny łącznie</label>
					<input
						id="hours_total"
						type="number"
						step="0.1"
						bind:value={form.hours_total}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="hours_sail" class="mb-1 block text-sm font-medium">Godziny żagli</label>
					<input
						id="hours_sail"
						type="number"
						step="0.1"
						bind:value={form.hours_sail}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="hours_engine" class="mb-1 block text-sm font-medium">Godziny silnika</label>
					<input
						id="hours_engine"
						type="number"
						step="0.1"
						bind:value={form.hours_engine}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="hours_6bf" class="mb-1 block text-sm font-medium">Godziny &gt;6Bf</label>
					<input
						id="hours_6bf"
						type="number"
						step="0.1"
						bind:value={form.hours_over_6bf}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="miles" class="mb-1 block text-sm font-medium">Mile</label>
					<input
						id="miles"
						type="number"
						step="0.1"
						bind:value={form.miles}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="days" class="mb-1 block text-sm font-medium">Dni</label>
					<input
						id="days"
						type="number"
						bind:value={form.days}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div class="flex items-end">
					<label class="flex items-center gap-2 text-sm">
						<input type="checkbox" bind:checked={form.tidal_waters} />
						Wody pływowe
					</label>
				</div>
			</div>
		{/if}

		<hr />
		<h3 class="font-semibold text-[var(--navy)]">Koszty</h3>
		<div class="grid grid-cols-2 gap-4">
			<div>
				<label for="cost_total" class="mb-1 block text-sm font-medium">Koszt całkowity</label>
				<input
					id="cost_total"
					type="number"
					step="0.01"
					bind:value={form.cost_total}
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
			<div>
				<label for="cost_pp" class="mb-1 block text-sm font-medium">Koszt na osobę</label>
				<input
					id="cost_pp"
					type="number"
					step="0.01"
					bind:value={form.cost_per_person}
					class="w-full rounded-lg border px-3 py-2"
				/>
			</div>
		</div>

		<div class="grid grid-cols-2 gap-4">
			<div>
				<label for="max_crew" class="mb-1 block text-sm font-medium">Maks. załoga</label>
				<input
					id="max_crew"
					type="number"
					bind:value={form.max_crew}
					class="w-full rounded-lg border px-3 py-2"
					min="0"
					placeholder="0 = bez limitu"
				/>
			</div>
		</div>

		<div>
			<label for="description" class="mb-1 block text-sm font-medium">Opis</label>
			<textarea
				id="description"
				bind:value={form.description}
				rows="4"
				class="w-full rounded-lg border px-3 py-2"
			></textarea>
		</div>

		<div class="flex gap-3">
			<button
				type="submit"
				disabled={loading}
				class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
			>
				{loading ? 'Tworzenie...' : 'Utwórz rejs'}
			</button>
			<a href="/cruises" class="rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50"
				>Anuluj</a
			>
		</div>
	</form>
</div>
