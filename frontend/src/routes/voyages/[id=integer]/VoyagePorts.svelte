<script lang="ts">
	import { addVoyagePort, deleteVoyagePort, reorderVoyagePorts } from '#lib/api/routes.js';
	import type { VoyagePortBody } from '#lib/api/types.js';
	import PortPicker from '#lib/components/PortPicker.svelte';

	import { onDestroy } from 'svelte';
	import type { PageData } from './$types';
	let active = true;
	onDestroy(() => {
		active = false;
	});
	function leftVoyage(voyageId: number) {
		return !active || voyageId !== id;
	}
	let {
		id,
		initialPorts,
		canLog
	}: { id: number; initialPorts: PageData['ports']; canLog: boolean } = $props();
	let ports = $derived(initialPorts);

	async function addPort(body: VoyagePortBody) {
		const voyageId = id;
		// Derive the next position from the max so it stays unique after deletions.
		const nextPosition = ports.length > 0 ? Math.max(...ports.map((p) => p.position)) + 1 : 0;
		const port = await addVoyagePort(voyageId, { ...body, position: nextPosition });
		if (leftVoyage(voyageId)) return;
		ports = [...ports, port];
	}

	async function removePort(index: number) {
		const voyageId = id;
		const port = ports[index];
		if (!port) return;
		await deleteVoyagePort(voyageId, port.id);
		if (leftVoyage(voyageId)) return;
		ports = ports.filter((p) => p.id !== port.id);
	}

	async function reorderPorts(from: number, to: number) {
		const voyageId = id;
		const next = [...ports];
		const [moved] = next.splice(from, 1);
		next.splice(to, 0, moved);
		// The endpoint returns the ports with their persisted positions.
		const saved =
			(await reorderVoyagePorts(
				voyageId,
				next.map((p) => p.id)
			)) ?? next;
		if (leftVoyage(voyageId)) return;
		ports = saved;
	}
</script>

<div class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
	<h2 class="mb-3 font-semibold text-[var(--navy)]">Odwiedzone porty ({ports.length})</h2>
	<!-- Keyed on the voyage so a link to another voyage starts the picker
	afresh: no typed search, alert or busy flag from the previous voyage. -->
	{#key id}
		<PortPicker
			{ports}
			readonly={!canLog}
			onAdd={addPort}
			onRemove={removePort}
			onReorder={reorderPorts}
		/>
	{/key}
</div>
