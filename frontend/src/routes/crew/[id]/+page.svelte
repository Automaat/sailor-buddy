<script lang="ts">
	import { api } from '$lib/api/client';
	import type { CrewMember } from '$lib/api/types';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let member = $state<CrewMember | null>(null);
	let loading = $state(true);

	const id = $derived(page.params.id);

	onMount(async () => {
		try {
			member = await api.get<CrewMember>(`/crew/${id}`);
		} catch (err) {
			console.error('Failed to load crew member:', err);
		} finally {
			loading = false;
		}
	});

	async function handleDelete() {
		if (!confirm('Delete this crew member?')) return;
		await api.del(`/crew/${id}`);
		goto('/crew');
	}
</script>

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Loading...</div>
{:else if member}
	<div class="mx-auto max-w-2xl">
		<div class="mb-6 flex items-center justify-between">
			<h1 class="text-3xl font-bold text-[var(--navy)]">{member.full_name}</h1>
			<button onclick={handleDelete} class="rounded-lg border border-red-200 px-4 py-2 text-sm text-red-600 hover:bg-red-50">
				Delete
			</button>
		</div>

		<div class="rounded-2xl bg-white p-6 shadow-sm">
			<dl class="grid grid-cols-2 gap-4 text-sm">
				{#if member.email}
					<dt class="text-[var(--text-muted)]">Email</dt>
					<dd>{member.email}</dd>
				{/if}
				{#if member.patent_number}
					<dt class="text-[var(--text-muted)]">Patent Number</dt>
					<dd>{member.patent_number}</dd>
				{/if}
			</dl>
		</div>
	</div>
{/if}
