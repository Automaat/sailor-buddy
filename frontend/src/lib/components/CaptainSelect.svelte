<script lang="ts">
	import { listMembers } from '#lib/api/routes.js';
	import type { Member } from '#lib/api/types.js';
	import { onMount } from 'svelte';

	// Picks the trip/voyage captain from the club's captain group. legacyName
	// is the free-text captain of imported or pre-group records; it is shown
	// until a captain from the group is chosen.
	type Props = {
		value: number;
		legacyName?: string;
		id?: string;
	};

	let { value = $bindable(), legacyName = '', id = 'captain' }: Props = $props();

	let captains = $state<Member[]>([]);

	onMount(async () => {
		try {
			const members = (await listMembers()) ?? [];
			captains = members.filter((m) => m.is_captain);
		} catch (err) {
			console.error('Failed to load captains:', err);
		}
	});
</script>

<select {id} bind:value class="w-full rounded-lg border px-3 py-2">
	<option value={0}>-- Brak --</option>
	{#each captains as captain}
		<option value={captain.id}>{captain.name}</option>
	{/each}
</select>
{#if !value && legacyName}
	<p class="mt-1 text-xs text-[var(--text-muted)]">Obecnie: {legacyName} (spoza kadry)</p>
{/if}
