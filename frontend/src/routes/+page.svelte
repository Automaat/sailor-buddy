<script lang="ts">
	import { api } from '$lib/api/client';
	import { orgStore } from '$lib/stores/org.svelte';
	import type { DashboardStats, OrgDashboardStats } from '$lib/api/types';

	let stats = $state<DashboardStats | OrgDashboardStats | null>(null);
	let loading = $state(true);

	async function load() {
		loading = true;
		try {
			stats = await api.get<DashboardStats>(`${orgStore.apiPrefix()}/dashboard`);
		} catch (err) {
			console.error('Failed to load dashboard:', err);
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
	<h1 class="mb-8 text-3xl font-bold text-[var(--navy)]">Pulpit</h1>

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if stats}
		<div class="mb-8 grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4">
			<div class="rounded-2xl bg-white p-6 shadow-sm">
				<div class="text-sm text-[var(--text-muted)]">Zrealizowane rejsy</div>
				<div class="mt-1 text-4xl font-bold text-[var(--navy)]">{stats.voyage_count}</div>
			</div>
			<div class="rounded-2xl bg-white p-6 shadow-sm">
				<div class="text-sm text-[var(--text-muted)]">Godziny na morzu</div>
				<div class="mt-1 text-4xl font-bold text-[var(--ocean)]">
					{Math.round(stats.total_hours)}
				</div>
				<div class="mt-1 text-xs text-[var(--text-muted)]">
					{Math.round(stats.total_hours_sail)}h żagle / {Math.round(stats.total_hours_engine)}h
					silnik
				</div>
			</div>
			<div class="rounded-2xl bg-white p-6 shadow-sm">
				<div class="text-sm text-[var(--text-muted)]">Mile morskie</div>
				<div class="mt-1 text-4xl font-bold text-[var(--sand)]">
					{Math.round(stats.total_miles).toLocaleString()}
				</div>
			</div>
			<div class="rounded-2xl bg-white p-6 shadow-sm">
				<div class="text-sm text-[var(--text-muted)]">Dni na morzu</div>
				<div class="mt-1 text-4xl font-bold text-[var(--navy)]">{stats.total_days}</div>
			</div>
		</div>

		{#if stats.by_year && stats.by_year.length > 0}
			<div class="rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-4 text-lg font-semibold text-[var(--navy)]">Wg roku</h2>
				<div class="overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b text-left text-[var(--text-muted)]">
								<th class="pb-2 pr-4">Rok</th>
								<th class="pb-2 pr-4">Rejsy</th>
								<th class="pb-2 pr-4">Godziny</th>
								<th class="pb-2 pr-4">Mile</th>
								<th class="pb-2">Dni</th>
							</tr>
						</thead>
						<tbody>
							{#each stats.by_year as row}
								<tr class="border-b border-gray-50">
									<td class="py-2 pr-4 font-medium">{row.year}</td>
									<td class="py-2 pr-4">{row.voyage_count}</td>
									<td class="py-2 pr-4">{Math.round(row.total_hours)}</td>
									<td class="py-2 pr-4">{Math.round(row.total_miles)}</td>
									<td class="py-2">{row.total_days}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>
		{/if}
	{/if}
</div>
