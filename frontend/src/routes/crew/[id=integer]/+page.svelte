<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { getCrew, deleteCrew } from '$lib/api/routes';
	import type { CrewMember } from '$lib/api/types';
	import { ApiError } from '$lib/api/client';
	import NotFound from '$lib/components/NotFound.svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let member = $state<CrewMember | null>(null);
	let loading = $state(true);
	let notFound = $state(false);

	const id = $derived(Number(page.params.id));

	onMount(async () => {
		try {
			member = await getCrew(id);
		} catch (err) {
			if (err instanceof ApiError && err.status === 404) {
				notFound = true;
			} else {
				console.error('Failed to load crew member:', err);
			}
		} finally {
			loading = false;
		}
	});

	async function handleDelete() {
		if (!confirm('Usunąć tego załoganta?')) return;
		await deleteCrew(id);
		goto('/crew');
	}
</script>

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else if member}
	<div class="mx-auto max-w-2xl">
		<div class="mb-6 flex items-center justify-between">
			<h1 class="text-3xl font-bold text-[var(--navy)]">{member.full_name}</h1>
			{#if auth.isAdmin}
				<button onclick={handleDelete} class="rounded-lg border border-red-200 px-4 py-2 text-sm text-red-600 hover:bg-red-50">
					Usuń
				</button>
			{/if}
		</div>

		<div class="rounded-2xl bg-white p-6 shadow-sm">
			<dl class="grid grid-cols-2 gap-4 text-sm">
				{#if member.email}
					<dt class="text-[var(--text-muted)]">Email</dt>
					<dd>{member.email}</dd>
				{/if}
				{#if member.patent_number}
					<dt class="text-[var(--text-muted)]">Numer patentu</dt>
					<dd>{member.patent_number}</dd>
				{/if}
			</dl>
		</div>
	</div>
{:else if notFound}
	<NotFound
		title="Nie znaleziono załoganta"
		message="Ten członek załogi nie istnieje lub nie masz do niego dostępu."
		backHref="/crew"
		backLabel="Cała załoga"
	/>
{:else}
	<NotFound
		title="Nie udało się wczytać załoganta"
		message="Coś poszło nie tak. Spróbuj ponownie za chwilę."
		backHref="/crew"
		backLabel="Cała załoga"
	/>
{/if}
