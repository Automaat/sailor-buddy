<script lang="ts">
	import { api } from '$lib/api/client';
	import { orgStore } from '$lib/stores/org.svelte';
	import type { Cruise } from '$lib/api/types';

	type Tab = 'trips' | 'voyages' | 'all';
	let activeTab = $state<Tab>('all');
	let cruises = $state<Cruise[]>([]);
	let loading = $state(true);

	async function load() {
		loading = true;
		try {
			const prefix = orgStore.apiPrefix();
			let endpoint: string;
			if (activeTab === 'trips') {
				endpoint = `${prefix}/trips`;
			} else if (activeTab === 'voyages') {
				endpoint = `${prefix}/voyages`;
			} else {
				endpoint = `${prefix}/cruises`;
			}
			cruises = await api.get<Cruise[]>(endpoint);
		} catch (err) {
			console.error('Failed to load cruises:', err);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		orgStore.currentSlug;
		activeTab;
		load();
	});

	function statusBadge(status: string) {
		switch (status) {
			case 'planned':
				return 'bg-blue-100 text-blue-700';
			case 'completed':
				return 'bg-green-100 text-green-700';
			case 'cancelled':
				return 'bg-gray-100 text-gray-500';
			default:
				return 'bg-gray-100 text-gray-500';
		}
	}

	function statusLabel(status: string) {
		switch (status) {
			case 'planned':
				return 'Planowany';
			case 'completed':
				return 'Zrealizowany';
			case 'cancelled':
				return 'Anulowany';
			default:
				return status;
		}
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-3xl font-bold text-[var(--navy)]">Rejsy</h1>
		<a
			href="/cruises/new"
			class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--ocean-dark)]"
		>
			+ Nowy rejs
		</a>
	</div>

	<div class="mb-4 flex gap-1 rounded-lg bg-gray-100 p-1">
		<button
			class="flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors {activeTab === 'all' ? 'bg-white shadow-sm text-[var(--navy)]' : 'text-[var(--text-muted)] hover:text-[var(--navy)]'}"
			onclick={() => (activeTab = 'all')}
		>
			Wszystkie
		</button>
		<button
			class="flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors {activeTab === 'trips' ? 'bg-white shadow-sm text-[var(--navy)]' : 'text-[var(--text-muted)] hover:text-[var(--navy)]'}"
			onclick={() => (activeTab = 'trips')}
		>
			Planowane
		</button>
		<button
			class="flex-1 rounded-md px-3 py-1.5 text-sm font-medium transition-colors {activeTab === 'voyages' ? 'bg-white shadow-sm text-[var(--navy)]' : 'text-[var(--text-muted)] hover:text-[var(--navy)]'}"
			onclick={() => (activeTab = 'voyages')}
		>
			Zrealizowane
		</button>
	</div>

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if cruises.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<p class="text-5xl">⛵</p>
			<p class="mt-4 text-lg text-[var(--text-muted)]">Brak rejsów</p>
			<a href="/cruises/new" class="mt-2 inline-block text-[var(--ocean)] hover:underline">
				Dodaj pierwszy rejs
			</a>
		</div>
	{:else}
		<div class="grid gap-4">
			{#each cruises as cruise}
				<a
					href="/cruises/{cruise.id}"
					class="flex items-center justify-between rounded-2xl bg-white p-6 shadow-sm transition-shadow hover:shadow-md"
				>
					<div>
						<div class="flex items-center gap-2">
							<h3 class="font-semibold text-[var(--navy)]">{cruise.name}</h3>
							<span class="rounded-full px-2 py-0.5 text-xs font-medium {statusBadge(cruise.status)}">
								{statusLabel(cruise.status)}
							</span>
						</div>
						<div class="mt-1 text-sm text-[var(--text-muted)]">
							{#if cruise.start_port && cruise.end_port}
								{cruise.start_port} → {cruise.end_port}
							{/if}
							{#if cruise.countries}
								· {cruise.countries}
							{/if}
						</div>
						<div class="mt-1 text-xs text-[var(--text-muted)]">
							{#if cruise.embark_date}
								{cruise.embark_date}
								{#if cruise.disembark_date}– {cruise.disembark_date}{/if}
							{/if}
						</div>
					</div>
					<div class="flex gap-6 text-right text-sm">
						{#if cruise.hours_total}
							<div>
								<div class="text-lg font-bold text-[var(--ocean)]">
									{Math.round(cruise.hours_total)}h
								</div>
								<div class="text-xs text-[var(--text-muted)]">godziny</div>
							</div>
						{/if}
						{#if cruise.miles}
							<div>
								<div class="text-lg font-bold text-[var(--sand)]">
									{Math.round(cruise.miles)}
								</div>
								<div class="text-xs text-[var(--text-muted)]">Mm</div>
							</div>
						{/if}
						{#if cruise.days}
							<div>
								<div class="text-lg font-bold text-[var(--navy)]">{cruise.days}</div>
								<div class="text-xs text-[var(--text-muted)]">dni</div>
							</div>
						{/if}
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>
