<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { listCruises } from '#lib/api/routes.js';
	import type { Cruise } from '#lib/api/types.js';
	import { cruiseKinds, cruiseKindLabels, type CruiseKind } from '#lib/cruise.js';
	import Sailboat from '@lucide/svelte/icons/sailboat';

	let cruises = $state<Cruise[]>([]);
	let loading = $state(true);
	let kindFilter = $state<CruiseKind | 'all'>('all');

	const visible = $derived(
		kindFilter === 'all' ? cruises : cruises.filter((c) => c.kind === kindFilter)
	);
	const filters: { value: CruiseKind | 'all'; label: string }[] = [
		{ value: 'all', label: 'Wszystkie' },
		...cruiseKinds.map((k) => ({ value: k, label: cruiseKindLabels[k] }))
	];

	async function load() {
		loading = true;
		try {
			cruises = await listCruises();
		} catch (err) {
			console.error('Failed to load cruises:', err);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
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
		{#if auth.isAdmin}
			<a
				href="/cruises/new"
				class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--ocean-dark)]"
			>
				+ Nowe wydarzenie
			</a>
		{/if}
	</div>

	<div class="mb-4 flex gap-2" role="group" aria-label="Rodzaj rejsu">
		{#each filters as f (f.value)}
			<button
				type="button"
				aria-pressed={kindFilter === f.value}
				onclick={() => (kindFilter = f.value)}
				class="rounded-full border px-3 py-1 text-sm transition-colors {kindFilter === f.value
					? 'border-[var(--ocean)] bg-[var(--ocean)] text-white'
					: 'bg-white text-[var(--text-muted)] hover:bg-gray-50'}"
			>
				{f.label}
			</button>
		{/each}
	</div>

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if cruises.length > 0 && visible.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<p class="text-lg text-[var(--text-muted)]">Brak wydarzeń tego rodzaju</p>
		</div>
	{:else if cruises.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Sailboat class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p class="mt-4 text-lg text-[var(--text-muted)]">Brak wydarzeń</p>
			{#if auth.isAdmin}
				<a href="/cruises/new" class="mt-2 inline-block text-[var(--ocean)] hover:underline">
					Utwórz pierwsze wydarzenie
				</a>
			{/if}
		</div>
	{:else}
		<div class="grid gap-4">
			{#each visible as cruise (cruise.id)}
				<a
					href="/cruises/{cruise.id}"
					class="flex items-center justify-between rounded-2xl bg-white p-6 shadow-sm transition-shadow hover:shadow-md"
				>
					<div>
						<div class="flex items-center gap-2">
							<h3 class="font-semibold text-[var(--navy)]">{cruise.name}</h3>
							{#if cruise.kind}
								<span class="rounded-full bg-[var(--ocean)]/10 px-2 py-0.5 text-xs text-[var(--ocean)]">
									{cruiseKindLabels[cruise.kind]}
								</span>
							{/if}
						</div>
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
