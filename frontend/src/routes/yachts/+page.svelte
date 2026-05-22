<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { listYachts, createYacht, deleteYacht } from '$lib/api/routes';
	import type { Yacht } from '$lib/api/types';
	import Ship from '@lucide/svelte/icons/ship';

	let yachts = $state<Yacht[]>([]);
	let loading = $state(true);
	let showForm = $state(false);
	let form = $state({ name: '', registration_no: '', yacht_type: '' });
	let saving = $state(false);

	async function load() {
		loading = true;
		try {
			yachts = await listYachts();
		} catch (err) {
			console.error('Failed to load yachts:', err);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		load();
	});

	async function handleAdd(e: Event) {
		e.preventDefault();
		saving = true;
		try {
			const yacht = await createYacht(form);
			yachts = [...yachts, yacht];
			form = { name: '', registration_no: '', yacht_type: '' };
			showForm = false;
		} catch (err) {
			console.error('Failed to add yacht:', err);
		} finally {
			saving = false;
		}
	}

	async function handleDelete(id: number) {
		if (!confirm('Usunąć ten jacht?')) return;
		await deleteYacht(id);
		yachts = yachts.filter((y) => y.id !== id);
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-3xl font-bold text-[var(--navy)]">Jachty</h1>
		{#if auth.isAdmin}
			<button onclick={() => (showForm = !showForm)} class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--ocean-dark)]">
				{showForm ? 'Anuluj' : '+ Dodaj jacht'}
			</button>
		{/if}
	</div>

	{#if showForm && auth.isAdmin}
		<form onsubmit={handleAdd} class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="grid grid-cols-3 gap-4">
				<div>
					<label for="name" class="mb-1 block text-sm font-medium">Nazwa *</label>
					<input id="name" type="text" bind:value={form.name} required class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="reg" class="mb-1 block text-sm font-medium">Nr rejestracyjny</label>
					<input id="reg" type="text" bind:value={form.registration_no} class="w-full rounded-lg border px-3 py-2" />
				</div>
				<div>
					<label for="type" class="mb-1 block text-sm font-medium">Typ</label>
					<input id="type" type="text" bind:value={form.yacht_type} class="w-full rounded-lg border px-3 py-2" />
				</div>
			</div>
			<button type="submit" disabled={saving} class="mt-4 rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
				{saving ? 'Dodawanie...' : 'Dodaj'}
			</button>
		</form>
	{/if}

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if yachts.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Ship class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p class="mt-4 text-lg text-[var(--text-muted)]">Brak jachtów</p>
		</div>
	{:else}
		<div class="grid gap-3">
			{#each yachts as yacht}
				<div class="flex items-center justify-between rounded-2xl bg-white px-6 py-4 shadow-sm">
					<div>
						<span class="font-semibold text-[var(--navy)]">{yacht.name}</span>
						{#if yacht.yacht_type}
							<span class="ml-2 text-sm text-[var(--text-muted)]">{yacht.yacht_type}</span>
						{/if}
					</div>
					<div class="flex items-center gap-4">
						{#if yacht.registration_no}
							<span class="text-sm text-[var(--text-muted)]">{yacht.registration_no}</span>
						{/if}
						{#if auth.isAdmin}
							<button onclick={() => handleDelete(yacht.id)} class="text-sm text-red-400 hover:text-red-600">Usuń</button>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>
