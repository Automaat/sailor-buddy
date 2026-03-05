<script lang="ts">
	import { api } from '$lib/api/client';
	import type { Cruise } from '$lib/api/types';
	import { onMount } from 'svelte';

	let cruises = $state<Cruise[]>([]);
	let loading = $state(true);

	onMount(async () => {
		try {
			cruises = await api.get<Cruise[]>('/cruises');
		} catch (err) {
			console.error('Failed to load cruises:', err);
		} finally {
			loading = false;
		}
	});
</script>

<div>
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-3xl font-bold text-[var(--navy)]">Cruises</h1>
		<a
			href="/cruises/new"
			class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--ocean-dark)]"
		>
			+ New Cruise
		</a>
	</div>

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Loading...</div>
	{:else if cruises.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<p class="text-5xl">⛵</p>
			<p class="mt-4 text-lg text-[var(--text-muted)]">No cruises yet</p>
			<a href="/cruises/new" class="mt-2 inline-block text-[var(--ocean)] hover:underline">
				Add your first cruise
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
						<h3 class="font-semibold text-[var(--navy)]">{cruise.name}</h3>
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
								<div class="text-xs text-[var(--text-muted)]">hours</div>
							</div>
						{/if}
						{#if cruise.miles}
							<div>
								<div class="text-lg font-bold text-[var(--sand)]">
									{Math.round(cruise.miles)}
								</div>
								<div class="text-xs text-[var(--text-muted)]">nm</div>
							</div>
						{/if}
						{#if cruise.days}
							<div>
								<div class="text-lg font-bold text-[var(--navy)]">{cruise.days}</div>
								<div class="text-xs text-[var(--text-muted)]">days</div>
							</div>
						{/if}
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>
