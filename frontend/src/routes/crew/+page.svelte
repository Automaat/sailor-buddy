<script lang="ts">
	import { api } from '$lib/api/client';
	import type { CrewMember } from '$lib/api/types';
	import { onMount } from 'svelte';

	let members = $state<CrewMember[]>([]);
	let loading = $state(true);
	let showForm = $state(false);
	let form = $state({ full_name: '', email: '', patent_number: '' });
	let saving = $state(false);

	onMount(async () => {
		try {
			members = await api.get<CrewMember[]>('/crew');
		} catch (err) {
			console.error('Failed to load crew:', err);
		} finally {
			loading = false;
		}
	});

	async function handleAdd(e: Event) {
		e.preventDefault();
		saving = true;
		try {
			const member = await api.post<CrewMember>('/crew', form);
			members = [...members, member];
			form = { full_name: '', email: '', patent_number: '' };
			showForm = false;
		} catch (err) {
			console.error('Failed to add crew member:', err);
		} finally {
			saving = false;
		}
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-3xl font-bold text-[var(--navy)]">Crew</h1>
		<button
			onclick={() => (showForm = !showForm)}
			class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--ocean-dark)]"
		>
			{showForm ? 'Cancel' : '+ Add Crew Member'}
		</button>
	</div>

	{#if showForm}
		<form onsubmit={handleAdd} class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="grid grid-cols-3 gap-4">
				<div>
					<label for="name" class="mb-1 block text-sm font-medium">Full Name *</label>
					<input id="name" type="text" bind:value={form.full_name} required class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="email" class="mb-1 block text-sm font-medium">Email</label>
					<input id="email" type="email" bind:value={form.email} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="patent" class="mb-1 block text-sm font-medium">Patent Number</label>
					<input id="patent" type="text" bind:value={form.patent_number} class="w-full rounded-lg border px-3 py-2" />
				</div>
			</div>
			<button type="submit" disabled={saving} class="mt-4 rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
				{saving ? 'Adding...' : 'Add'}
			</button>
		</form>
	{/if}

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Loading...</div>
	{:else if members.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<p class="text-5xl">👥</p>
			<p class="mt-4 text-lg text-[var(--text-muted)]">No crew members yet</p>
		</div>
	{:else}
		<div class="grid gap-3">
			{#each members as member}
				<a href="/crew/{member.id}" class="flex items-center justify-between rounded-2xl bg-white px-6 py-4 shadow-sm transition-shadow hover:shadow-md">
					<div>
						<span class="font-semibold text-[var(--navy)]">{member.full_name}</span>
						{#if member.email}
							<span class="ml-2 text-sm text-[var(--text-muted)]">{member.email}</span>
						{/if}
					</div>
					{#if member.patent_number}
						<span class="text-sm text-[var(--text-muted)]">Patent: {member.patent_number}</span>
					{/if}
				</a>
			{/each}
		</div>
	{/if}
</div>
