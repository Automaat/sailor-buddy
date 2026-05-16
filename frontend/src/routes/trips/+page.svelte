<script lang="ts">
	import { api } from '$lib/api/client';
	import { orgStore } from '$lib/stores/org.svelte';
	import type { Trip } from '$lib/api/types';
	import Sailboat from '@lucide/svelte/icons/sailboat';

	let trips = $state<Trip[]>([]);
	let loading = $state(true);

	async function load() {
		loading = true;
		try {
			trips = await api.list<Trip>(`${orgStore.apiPrefix()}/trips`);
		} catch (err) {
			console.error('Failed to load trips:', err);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		orgStore.currentSlug;
		load();
	});

	function statusBadge(status: string) {
		return status === 'cancelled' ? 'bg-gray-100 text-gray-500' : 'bg-blue-100 text-blue-700';
	}

	function statusLabel(status: string) {
		return status === 'cancelled' ? 'Anulowany' : 'Planowany';
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between">
		<div>
			<h1 class="text-3xl font-bold text-[var(--navy)]">Planowane rejsy</h1>
			<p class="mt-1 text-sm text-[var(--text-muted)]">
				<a href="/voyages" class="text-[var(--ocean)] hover:underline">Zobacz zrealizowane →</a>
			</p>
		</div>
		<a
			href="/trips/new"
			class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--ocean-dark)]"
		>
			+ Zaplanuj rejs
		</a>
	</div>

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if trips.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Sailboat class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p class="mt-4 text-lg text-[var(--text-muted)]">Brak planowanych rejsów</p>
			<a href="/trips/new" class="mt-2 inline-block text-[var(--ocean)] hover:underline">
				Zaplanuj pierwszy rejs
			</a>
		</div>
	{:else}
		<div class="grid gap-4">
			{#each trips as trip}
				<a
					href="/trips/{trip.id}"
					class="flex items-center justify-between rounded-2xl bg-white p-6 shadow-sm transition-shadow hover:shadow-md"
				>
					<div>
						<div class="flex items-center gap-2">
							<h3 class="font-semibold text-[var(--navy)]">{trip.name}</h3>
							<span class="rounded-full px-2 py-0.5 text-xs font-medium {statusBadge(trip.status)}">
								{statusLabel(trip.status)}
							</span>
						</div>
						<div class="mt-1 text-sm text-[var(--text-muted)]">
							{#if trip.start_port && trip.end_port}
								{trip.start_port} → {trip.end_port}
							{/if}
							{#if trip.countries} · {trip.countries}{/if}
						</div>
						<div class="mt-1 text-xs text-[var(--text-muted)]">
							{#if trip.embark_date}
								{trip.embark_date}
								{#if trip.disembark_date}– {trip.disembark_date}{/if}
							{/if}
						</div>
					</div>
					<div class="text-right text-sm text-[var(--text-muted)]">
						{#if trip.captain_name}<div>kpt. {trip.captain_name}</div>{/if}
						{#if trip.max_crew}<div>max {trip.max_crew} os.</div>{/if}
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>
