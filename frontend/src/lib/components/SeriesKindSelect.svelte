<script lang="ts">
	import { cruiseKinds, cruiseKindLabels, type CruiseKind } from '#lib/cruise.js';

	let { kind = $bindable<CruiseKind>('sea') }: { kind?: CruiseKind } = $props();
</script>

<div>
	<label for="kind" class="mb-1 block text-sm font-medium">Rodzaj rejsów</label>
	<select id="kind" bind:value={kind} class="w-full rounded-lg border px-3 py-2">
		{#each cruiseKinds as k (k)}
			<option value={k}>{cruiseKindLabels[k]}</option>
		{/each}
	</select>
</div>
