<script module lang="ts">
	import type { Cruise } from '#lib/api/types.js';

	/** The picker's first step: a standalone trip, cruises without a series, or a series id. */
	export const STANDALONE = 'standalone';
	export const NO_SERIES = 'none';

	/** The first-step choice that holds cruise `cruiseID`; standalone for 0. */
	export function groupOf(cruiseID: number, cruises: Cruise[]): string {
		if (!cruiseID) return STANDALONE;
		const seriesID = cruises.find((c) => c.id === cruiseID)?.series_id;
		return seriesID ? String(seriesID) : NO_SERIES;
	}
</script>

<script lang="ts">
	import type { CruiseSeries } from '#lib/api/types.js';

	/**
	 * Optional two-step edition picker of the trip form: Seria, then Edycja
	 * (the cruises of that series). The default is a standalone trip with no
	 * edition; cruises without a series are under "Bez serii". `locked` keeps
	 * the edition a link preselected. A linked edition missing from `cruises`
	 * (failed load) stays shown so a save keeps it.
	 */
	type Props = {
		group: string;
		value: number;
		cruises: Cruise[];
		series: CruiseSeries[];
		locked?: boolean;
		onchange?: (cruiseID: number) => void;
	};

	let {
		group = $bindable(),
		value = $bindable(),
		cruises,
		series,
		locked = false,
		onchange
	}: Props = $props();

	const editions = $derived(
		group === NO_SERIES
			? cruises.filter((c) => c.series_id == null)
			: cruises.filter((c) => String(c.series_id) === group)
	);
	const missingLinked = $derived(value > 0 && !editions.some((c) => c.id === value));

	function label(c: Cruise): string {
		return c.edition ? `${c.edition} — ${c.name}` : c.name;
	}

	function pickGroup() {
		value = 0;
	}
</script>

<div>
	<label for="series" class="mb-1 block text-sm font-medium">Seria</label>
	<select
		id="series"
		bind:value={group}
		onchange={pickGroup}
		disabled={locked}
		class="w-full rounded-lg border px-3 py-2"
	>
		<option value={STANDALONE}>Samodzielny (bez edycji)</option>
		{#each series as s (s.id)}
			<option value={String(s.id)}>{s.name}</option>
		{/each}
		<option value={NO_SERIES}>Bez serii</option>
	</select>
</div>
{#if group !== STANDALONE}
	<div>
		<label for="cruise" class="mb-1 block text-sm font-medium">Edycja *</label>
		<select
			id="cruise"
			bind:value
			onchange={(e) => onchange?.(Number(e.currentTarget.value))}
			disabled={locked}
			required
			aria-required="true"
			class="w-full rounded-lg border px-3 py-2"
		>
			<option value={0} disabled>-- Wybierz edycję --</option>
			{#if missingLinked}
				<option {value}>Wydarzenie #{value}</option>
			{/if}
			{#each editions as cruise (cruise.id)}
				<option value={cruise.id}>{label(cruise)}</option>
			{/each}
		</select>
	</div>
{/if}
