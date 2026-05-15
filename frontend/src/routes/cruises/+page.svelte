<script lang="ts">
	import { api } from '$lib/api/client';
	import { orgStore } from '$lib/stores/org.svelte';
	import type { Cruise } from '$lib/api/types';
	import Sailboat from '@lucide/svelte/icons/sailboat';
	import { goto } from '$app/navigation';

	let cruises = $state<Cruise[]>([]);
	let loading = $state(true);

	async function load() {
		if (!orgStore.isOrgMode) {
			cruises = [];
			loading = false;
			return;
		}
		loading = true;
		try {
			cruises = await api.get<Cruise[]>(`${orgStore.apiPrefix()}/cruises`);
		} catch (err) {
			console.error('Failed to load cruises:', err);
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
			<h1 class="text-3xl font-bold text-[var(--navy)]">Wydarzenia klubu</h1>
			<p class="mt-1 text-sm text-[var(--text-muted)]">
				Rejsy wieloyachtowe z otwartymi zapisami
			</p>
		</div>
		{#if orgStore.isOrgMode && orgStore.isOrgAdmin}
			<a
				href="/cruises/new"
				class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--ocean-dark)]"
			>
				+ Nowe wydarzenie
			</a>
		{/if}
	</div>

	{#if !orgStore.isOrgMode}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Sailboat class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p class="mt-4 text-lg text-[var(--text-muted)]">Wydarzenia istnieją tylko w klubie</p>
			<button
				onclick={() => goto('/orgs')}
				class="mt-2 text-[var(--ocean)] hover:underline"
			>
				Przejdź do klubów
			</button>
		</div>
	{:else if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if cruises.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Sailboat class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p class="mt-4 text-lg text-[var(--text-muted)]">Brak wydarzeń</p>
			{#if orgStore.isOrgAdmin}
				<a href="/cruises/new" class="mt-2 inline-block text-[var(--ocean)] hover:underline">
					Utwórz pierwsze wydarzenie
				</a>
			{/if}
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
							{#if cruise.countries} · {cruise.countries}{/if}
						</div>
						<div class="mt-1 text-xs text-[var(--text-muted)]">
							{#if cruise.embark_date}
								{cruise.embark_date}
								{#if cruise.disembark_date}– {cruise.disembark_date}{/if}
							{/if}
						</div>
					</div>
					<div class="text-right text-sm text-[var(--text-muted)]">
						{#if cruise.max_crew}<div>max {cruise.max_crew} os.</div>{/if}
						{#if cruise.enroll_token}
							<div class="mt-1 inline-block rounded-full bg-green-100 px-2 py-0.5 text-xs text-green-700">
								Zapisy otwarte
							</div>
						{/if}
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>
