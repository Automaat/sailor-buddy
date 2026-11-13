<script lang="ts">
	import type { Cruise } from '#lib/api/types.js';

	/**
	 * Required club-event picker for trip and voyage forms. A linked event that
	 * is missing from `cruises` (failed load) stays selectable so a save keeps it.
	 */
	type Props = {
		value: number;
		cruises: Cruise[];
		id?: string;
		onchange?: (cruiseID: number) => void;
	};

	let { value = $bindable(), cruises, id = 'cruise', onchange }: Props = $props();

	const missingLinked = $derived(value > 0 && !cruises.some((c) => c.id === value));
</script>

<select
	{id}
	bind:value
	onchange={(e) => onchange?.(Number(e.currentTarget.value))}
	required
	aria-required="true"
	class="w-full rounded-lg border px-3 py-2"
>
	<option value={0} disabled>-- Wybierz wydarzenie --</option>
	{#if missingLinked}
		<option value={value}>Wydarzenie #{value}</option>
	{/if}
	{#each cruises as cruise (cruise.id)}
		<option value={cruise.id}>{cruise.name}</option>
	{/each}
</select>
