<script lang="ts">
	import { onMount } from 'svelte';
	import { listSeries } from '#lib/api/routes.js';
	import type { CruiseSeries } from '#lib/api/types.js';

	let {
		seriesId = $bindable<number | null>(null),
		edition = $bindable('')
	}: { seriesId?: number | null; edition?: string } = $props();

	let series = $state<CruiseSeries[]>([]);

	onMount(async () => {
		try {
			series = await listSeries();
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
