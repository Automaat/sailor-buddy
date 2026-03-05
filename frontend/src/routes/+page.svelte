<script lang="ts">
	import { api } from '$lib/api/client';
	import type { DashboardStats } from '$lib/api/types';
	import { onMount } from 'svelte';

	let stats = $state<DashboardStats | null>(null);
	let loading = $state(true);

	onMount(async () => {
		try {
			stats = await api.get<DashboardStats>('/dashboard');
		} catch (err) {
			console.error('Failed to load dashboard:', err);
		} finally {
			loading = false;
		}
	});
</script>

<div>
	<h1 class="mb-8 text-3xl font-bold text-[var(--navy)]">Dashboard</h1>

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Loading...</div>
	{:else if stats}
		<div class="mb-8 grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4">
			<div class="rounded-2xl bg-white p-6 shadow-sm">
				<div class="text-sm text-[var(--text-muted)]">Total Cruises</div>
				<div class="mt-1 text-4xl font-bold text-[var(--navy)]">{stats.cruise_count}</div>
			</div>
			<div class="rounded-2xl bg-white p-6 shadow-sm">
				<div class="text-sm text-[var(--text-muted)]">Hours at Sea</div>
				<div class="mt-1 text-4xl font-bold text-[var(--ocean)]">
					{Math.round(stats.total_hours)}
				</div>
				<div class="mt-1 text-xs text-[var(--text-muted)]">
					{Math.round(stats.total_hours_sail)}h sail / {Math.round(stats.total_hours_engine)}h
					engine
				</div>
			</div>
			<div class="rounded-2xl bg-white p-6 shadow-sm">
				<div class="text-sm text-[var(--text-muted)]">Nautical Miles</div>
				<div class="mt-1 text-4xl font-bold text-[var(--sand)]">
					{Math.round(stats.total_miles).toLocaleString()}
				</div>
			</div>
			<div class="rounded-2xl bg-white p-6 shadow-sm">
				<div class="text-sm text-[var(--text-muted)]">Days at Sea</div>
				<div class="mt-1 text-4xl font-bold text-[var(--navy)]">{stats.total_days}</div>
			</div>
		</div>

		{#if stats.by_year && stats.by_year.length > 0}
			<div class="rounded-2xl bg-white p-6 shadow-sm">
				<h2 class="mb-4 text-lg font-semibold text-[var(--navy)]">By Year</h2>
				<div class="overflow-x-auto">
					<table class="w-full text-sm">
						<thead>
							<tr class="border-b text-left text-[var(--text-muted)]">
								<th class="pb-2 pr-4">Year</th>
								<th class="pb-2 pr-4">Cruises</th>
								<th class="pb-2 pr-4">Hours</th>
								<th class="pb-2 pr-4">Miles</th>
								<th class="pb-2">Days</th>
							</tr>
						</thead>
						<tbody>
							{#each stats.by_year as row}
								<tr class="border-b border-gray-50">
									<td class="py-2 pr-4 font-medium">{row.year}</td>
									<td class="py-2 pr-4">{row.cruise_count}</td>
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
