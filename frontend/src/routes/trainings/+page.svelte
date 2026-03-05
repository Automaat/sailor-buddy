<script lang="ts">
	import { api } from '$lib/api/client';
	import type { Training } from '$lib/api/types';
	import { onMount } from 'svelte';

	let trainings = $state<Training[]>([]);
	let loading = $state(true);
	let showForm = $state(false);
	let form = $state({ name: '', date: '', organizer: '', cost: 0, url: '' });
	let saving = $state(false);

	onMount(async () => {
		try {
			trainings = await api.get<Training[]>('/trainings');
		} catch (err) {
			console.error('Failed to load trainings:', err);
		} finally {
			loading = false;
		}
	});

	async function handleAdd(e: Event) {
		e.preventDefault();
		saving = true;
		try {
			const training = await api.post<Training>('/trainings', form);
			trainings = [training, ...trainings];
			form = { name: '', date: '', organizer: '', cost: 0, url: '' };
			showForm = false;
		} catch (err) {
			console.error('Failed to add training:', err);
		} finally {
			saving = false;
		}
	}

	async function handleDelete(id: number) {
		if (!confirm('Delete this training?')) return;
		await api.del(`/trainings/${id}`);
		trainings = trainings.filter((t) => t.id !== id);
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-3xl font-bold text-[var(--navy)]">Trainings</h1>
		<button onclick={() => (showForm = !showForm)} class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--ocean-dark)]">
			{showForm ? 'Cancel' : '+ Add Training'}
		</button>
	</div>

	{#if showForm}
		<form onsubmit={handleAdd} class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="grid grid-cols-2 gap-4">
				<div>
					<label for="name" class="mb-1 block text-sm font-medium">Name *</label>
					<input id="name" type="text" bind:value={form.name} required class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="date" class="mb-1 block text-sm font-medium">Date</label>
					<input id="date" type="date" bind:value={form.date} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="org" class="mb-1 block text-sm font-medium">Organizer</label>
					<input id="org" type="text" bind:value={form.organizer} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="cost" class="mb-1 block text-sm font-medium">Cost</label>
					<input id="cost" type="number" step="0.01" bind:value={form.cost} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div class="col-span-2">
					<label for="url" class="mb-1 block text-sm font-medium">URL</label>
					<input id="url" type="url" bind:value={form.url} class="w-full rounded-lg border px-3 py-2" />
				</div>
			</div>
			<button type="submit" disabled={saving} class="mt-4 rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
				{saving ? 'Adding...' : 'Add'}
			</button>
		</form>
	{/if}

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Loading...</div>
	{:else if trainings.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<p class="text-5xl">📋</p>
			<p class="mt-4 text-lg text-[var(--text-muted)]">No trainings yet</p>
		</div>
	{:else}
		<div class="grid gap-3">
			{#each trainings as training}
				<div class="flex items-center justify-between rounded-2xl bg-white px-6 py-4 shadow-sm">
					<div>
						<span class="font-semibold text-[var(--navy)]">{training.name}</span>
						{#if training.organizer}
							<span class="ml-2 text-sm text-[var(--text-muted)]">by {training.organizer}</span>
						{/if}
						{#if training.date}
							<span class="ml-2 text-sm text-[var(--text-muted)]">{training.date}</span>
						{/if}
					</div>
					<div class="flex items-center gap-4">
						{#if training.cost}
							<span class="text-sm font-medium text-[var(--sand)]">{training.cost} PLN</span>
						{/if}
						{#if training.url}
							<a href={training.url} target="_blank" class="text-sm text-[var(--ocean)] hover:underline">Link</a>
						{/if}
						<button onclick={() => handleDelete(training.id)} class="text-sm text-red-400 hover:text-red-600">Delete</button>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>
