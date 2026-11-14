<script lang="ts">
	import { onMount } from 'svelte';
	import { listSeries } from '#lib/api/routes.js';
	import type { CruiseSeries } from '#lib/api/types.js';

	/**
	 * Series picker and edition field of a cruise form. `series` is the list a
	 * page's load function fetched; without it the picker fetches the list
	 * itself on mount.
	 */
	let {
		seriesId = $bindable<number | null>(null),
		edition = $bindable(''),
		series: loaded
	}: { seriesId?: number | null; edition?: string; series?: CruiseSeries[] } = $props();

	let fetched = $state<CruiseSeries[]>([]);
	const series = $derived(loaded ?? fetched);

	onMount(async () => {
		if (loaded) return;
		try {
			fetched = await listSeries();
		} catch (err) {
			console.error('Failed to load series:', err);
		}
	});
</script>

<div>
	<label for="series" class="mb-1 block text-sm font-medium">Seria</label>
	<select id="series" bind:value={seriesId} class="w-full rounded-lg border px-3 py-2">
		<option value={null}>Bez serii</option>
		{#each series as s (s.id)}
			<option value={s.id}>{s.name}</option>
		{/each}
	</select>
</div>
<div>
	<label for="edition" class="mb-1 block text-sm font-medium">Edycja</label>
	<input
		id="edition"
		type="text"
		bind:value={edition}
		maxlength="50"
		placeholder="np. 2025 lub 2026/27"
		class="w-full rounded-lg border px-3 py-2"
	/>
</div>
