<script lang="ts">
	import { api } from '$lib/api/client';
	import { orgStore } from '$lib/stores/org.svelte';
	import type { Voyage } from '$lib/api/types';
	import Sailboat from '@lucide/svelte/icons/sailboat';

	let voyages = $state<Voyage[]>([]);
	let loading = $state(true);

	async function load() {
		loading = true;
		try {
			voyages = await api.get<Voyage[]>(`${orgStore.apiPrefix()}/voyages`);
		} catch (err) {
			console.error('Failed to load voyages:', err);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		orgStore.currentSlug;
		load();
	});
</script>

<div>
	<div class="mb-6 flex items-center justify-between">
		<div>
			<h1 class="text-3xl font-bold text-[var(--navy)]">Zrealizowane rejsy</h1>
			<p class="mt-1 text-sm text-[var(--text-muted)]">
				<a href="/trips" class="text-[var(--ocean)] hover:underline">Zobacz planowane →</a>
			</p>
		</div>
		<a
			href="/voyages/new"
			class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--ocean-dark)]"
		>
			+ Wpisz rejs
		</a>
	</div>

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if voyages.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Sailboat class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p class="mt-4 text-lg text-[var(--text-muted)]">Brak zrealizowanych rejsów</p>
			<a href="/voyages/new" class="mt-2 inline-block text-[var(--ocean)] hover:underline">
				Wpisz pierwszy rejs
			</a>
		</div>
	{:else}
		<div class="grid gap-4">
			{#each voyages as voyage}
				<a
					href="/voyages/{voyage.id}"
					class="flex items-center justify-between rounded-2xl bg-white p-6 shadow-sm transition-shadow hover:shadow-md"
				>
					<div>
						<h3 class="font-semibold text-[var(--navy)]">{voyage.name}</h3>
						<div class="mt-1 text-sm text-[var(--text-muted)]">
							{#if voyage.start_port && voyage.end_port}
								{voyage.start_port} → {voyage.end_port}
							{/if}
							{#if voyage.countries} · {voyage.countries}{/if}
						</div>
						<div class="mt-1 text-xs text-[var(--text-muted)]">
							{#if voyage.embark_date}
								{voyage.embark_date}
								{#if voyage.disembark_date}– {voyage.disembark_date}{/if}
							{/if}
							{#if voyage.year} · {voyage.year}{/if}
						</div>
					</div>
					<div class="flex gap-6 text-right text-sm">
						<div>
							<div class="text-lg font-bold text-[var(--ocean)]">{Math.round(voyage.hours_total)}h</div>
							<div class="text-xs text-[var(--text-muted)]">godziny</div>
						</div>
						<div>
							<div class="text-lg font-bold text-[var(--sand)]">{Math.round(voyage.miles)}</div>
							<div class="text-xs text-[var(--text-muted)]">Mm</div>
						</div>
						<div>
							<div class="text-lg font-bold text-[var(--navy)]">{voyage.days}</div>
							<div class="text-xs text-[var(--text-muted)]">dni</div>
						</div>
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>
