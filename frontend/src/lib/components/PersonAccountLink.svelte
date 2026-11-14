<script lang="ts">
	import { listMembers, linkCrewAccount } from '#lib/api/routes.js';
	import type { CrewMember, Member } from '#lib/api/types.js';
	import { ApiError } from '#lib/api/client.js';
	import { onMount } from 'svelte';

	let {
		person,
		onlinked,
		oncancel
	}: {
		person: CrewMember;
		onlinked: (person: CrewMember) => void;
		oncancel: () => void;
	} = $props();

	let members = $state<Member[]>([]);
	let userID = $state(0);
	let busy = $state(false);
	let error = $state('');

	onMount(async () => {
		try {
			members = (await listMembers()) ?? [];
		} catch (err) {
			console.error('Failed to load members:', err);
			error = 'Nie udało się wczytać listy kont.';
		}
	});

	async function link(e: Event) {
		e.preventDefault();
		if (!userID) return;
		busy = true;
		error = '';
		try {
			onlinked(await linkCrewAccount(person.id, userID));
		} catch (err) {
			if (err instanceof ApiError && err.status === 409) {
				error = 'Ta osoba ma już inne konto.';
			} else if (err instanceof ApiError && err.status === 422) {
				error = 'Nie można połączyć: po połączeniu byłoby za dużo innych nazwisk.';
			} else {
				error = 'Nie udało się połączyć z kontem.';
			}
		} finally {
			busy = false;
		}
	}
</script>

<form onsubmit={link} class="mt-6 rounded-2xl bg-white p-6 shadow-sm" aria-label="Połącz z kontem">
	<h2 class="mb-2 text-lg font-semibold text-[var(--navy)]">Połącz z kontem</h2>
	<p class="mb-3 text-sm text-[var(--text-muted)]">
		Dotychczasowa osoba konta zostanie połączona z {person.full_name}: jej rejsy i opinie przejdą
		tutaj.
	</p>
	<label for="link-account" class="mb-1 block text-sm font-medium">Konto</label>
	<select id="link-account" bind:value={userID} class="w-full rounded-lg border px-3 py-2 text-sm">
		<option value={0}>Wybierz konto…</option>
		{#each members as m (m.id)}
			<option value={m.id}>{m.name} ({m.email})</option>
		{/each}
	</select>
	{#if error}
		<p class="mt-3 text-sm text-red-600">{error}</p>
	{/if}
	<div class="mt-4 flex gap-2">
		<button
			type="submit"
			disabled={busy || !userID}
			class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
		>
			{busy ? 'Łączenie...' : 'Połącz'}
		</button>
		<button
			type="button"
			onclick={oncancel}
			disabled={busy}
			class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50 disabled:opacity-50"
		>
			Anuluj
		</button>
	</div>
</form>
