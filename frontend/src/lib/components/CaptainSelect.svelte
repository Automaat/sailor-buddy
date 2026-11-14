<script lang="ts">
	import { listCrew, listMembers } from '#lib/api/routes.js';
	import { onMount } from 'svelte';

	// Picks the trip/voyage captain from the club's people. People whose
	// account is in the captain group come first, under their own heading;
	// any person may be chosen. legacyName is the free-text captain of
	// imported records; it is shown until a person is chosen.
	type Props = {
		value: number;
		legacyName?: string;
		id?: string;
	};

	type Option = { id: number; name: string };

	let { value = $bindable(), legacyName = '', id = 'captain' }: Props = $props();

	let captains = $state<Option[]>([]);
	let others = $state<Option[]>([]);
	let loadFailed = $state(false);

	onMount(async () => {
		try {
			const [people, members] = await Promise.all([
				listCrew(),
				listMembers().catch((err) => {
					console.error('Failed to load the captain group:', err);
					return [];
				})
			]);
			const group = new Set((members ?? []).filter((m) => m.is_captain).map((m) => m.id));
			const byName = (a: Option, b: Option) => a.name.localeCompare(b.name, 'pl');
			const isCaptain = (userID?: number) => userID != null && group.has(userID);
			captains = (people ?? [])
				.filter((p) => isCaptain(p.user_id))
				.map((p) => ({ id: p.id, name: p.full_name }))
				.sort(byName);
			others = (people ?? [])
				.filter((p) => !isCaptain(p.user_id))
				.map((p) => ({ id: p.id, name: p.full_name }))
				.sort(byName);
		} catch (err) {
			console.error('Failed to load people:', err);
			loadFailed = true;
		}
	});
</script>

<select {id} bind:value class="w-full rounded-lg border px-3 py-2">
	<option value={0}>-- Brak --</option>
	{#if captains.length}
		<optgroup label="Kapitanowie">
			{#each captains as person (person.id)}
				<option value={person.id}>{person.name}</option>
			{/each}
		</optgroup>
	{/if}
	{#if others.length}
		<optgroup label="Pozostałe osoby">
			{#each others as person (person.id)}
				<option value={person.id}>{person.name}</option>
			{/each}
		</optgroup>
	{/if}
</select>
{#if loadFailed}
	<p role="alert" class="mt-1 text-xs text-red-700">
		Nie udało się wczytać listy osób. Odśwież stronę, aby spróbować ponownie.
	</p>
{/if}
{#if !value && legacyName}
	<p class="mt-1 text-xs text-[var(--text-muted)]">Obecnie: {legacyName} (nie wybrano osoby)</p>
{/if}
