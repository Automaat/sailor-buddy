<script lang="ts">
	import { api } from '$lib/api/client';
	import { orgStore } from '$lib/stores/org.svelte';
	import type { Organization } from '$lib/api/types';
	import { goto } from '$app/navigation';

	let orgs = $state<Organization[]>([]);
	let loading = $state(true);
	let showCreate = $state(false);
	let creating = $state(false);
	let error = $state('');

	let form = $state({ name: '', slug: '', description: '', city: '', website: '', pzz_club_number: '' });

	async function load() {
		loading = true;
		try {
			orgs = await api.get<Organization[]>('/orgs');
		} finally {
			loading = false;
		}
	}

	function generateSlug(name: string): string {
		return name
			.toLowerCase()
			.replace(/[ąàáâãäå]/g, 'a')
			.replace(/[ćčç]/g, 'c')
			.replace(/[ęèéêë]/g, 'e')
			.replace(/[łľ]/g, 'l')
			.replace(/[ńñň]/g, 'n')
			.replace(/[óòôõö]/g, 'o')
			.replace(/[śšş]/g, 's')
			.replace(/[ůúùûü]/g, 'u')
			.replace(/[żźž]/g, 'z')
			.replace(/[^a-z0-9]+/g, '-')
			.replace(/^-|-$/g, '');
	}

	async function handleCreate(e: Event) {
		e.preventDefault();
		error = '';
		creating = true;
		try {
			await api.post('/orgs', {
				name: form.name,
				slug: form.slug,
				description: form.description || undefined,
				city: form.city || undefined,
				website: form.website || undefined,
				pzz_club_number: form.pzz_club_number || undefined
			});
			await orgStore.refresh();
			orgStore.select(form.slug);
			goto('/');
		} catch (e: any) {
			error = e.message;
		} finally {
			creating = false;
		}
	}

	$effect(() => {
		load();
	});
</script>

<div class="mx-auto max-w-4xl">
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-2xl font-bold text-[var(--navy)]">Kluby żeglarskie</h1>
		<button
			onclick={() => (showCreate = !showCreate)}
			class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-white transition-colors hover:bg-[var(--ocean)]/80"
		>
			{showCreate ? 'Anuluj' : 'Nowy klub'}
		</button>
	</div>

	{#if showCreate}
		<div class="mb-6 rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
			<h2 class="mb-4 text-lg font-semibold text-[var(--navy)]">Utwórz klub</h2>
			{#if error}
				<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
			{/if}
			<form onsubmit={handleCreate} class="space-y-4">
				<div class="grid grid-cols-2 gap-4">
					<div>
						<label class="mb-1 block text-sm font-medium text-gray-700" for="org-name"
							>Nazwa *</label
						>
						<input
							id="org-name"
							type="text"
							bind:value={form.name}
							oninput={() => (form.slug = generateSlug(form.name))}
							class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
							required
						/>
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium text-gray-700" for="org-slug"
							>Slug *</label
						>
						<input
							id="org-slug"
							type="text"
							bind:value={form.slug}
							class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
							required
							pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
						/>
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium text-gray-700" for="org-city"
							>Miasto</label
						>
						<input
							id="org-city"
							type="text"
							bind:value={form.city}
							class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
						/>
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium text-gray-700" for="org-pzz"
							>Nr klubu PZŻ</label
						>
						<input
							id="org-pzz"
							type="text"
							bind:value={form.pzz_club_number}
							class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
						/>
					</div>
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium text-gray-700" for="org-website"
						>Strona WWW</label
					>
					<input
						id="org-website"
						type="text"
						bind:value={form.website}
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
					/>
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium text-gray-700" for="org-desc"
						>Opis</label
					>
					<textarea
						id="org-desc"
						bind:value={form.description}
						rows="3"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
					></textarea>
				</div>
				<button
					type="submit"
					disabled={creating}
					class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-white transition-colors hover:bg-[var(--ocean)]/80 disabled:opacity-50"
				>
					{creating ? 'Tworzenie...' : 'Utwórz'}
				</button>
			</form>
		</div>
	{/if}

	{#if loading}
		<p class="text-gray-500">Ładowanie...</p>
	{:else if orgs.length === 0}
		<div class="rounded-lg border border-gray-200 bg-white p-8 text-center shadow-sm">
			<p class="text-gray-500">Nie należysz jeszcze do żadnego klubu.</p>
			<p class="mt-2 text-sm text-gray-400">Utwórz nowy lub dołącz przez link zaproszenia.</p>
		</div>
	{:else}
		<div class="grid gap-4 sm:grid-cols-2">
			{#each orgs as org}
				<button
					onclick={() => {
						orgStore.select(org.slug);
						goto('/');
					}}
					class="rounded-lg border border-gray-200 bg-white p-6 text-left shadow-sm transition-colors hover:border-[var(--ocean)]"
				>
					<div class="flex items-center justify-between">
						<h3 class="font-semibold text-[var(--navy)]">{org.name}</h3>
						<span
							class="rounded-full bg-[var(--ocean)]/10 px-2 py-0.5 text-xs text-[var(--ocean)]"
						>
							{org.role}
						</span>
					</div>
					{#if org.city}
						<p class="mt-1 text-sm text-gray-500">{org.city}</p>
					{/if}
					{#if org.description}
						<p class="mt-2 line-clamp-2 text-sm text-gray-600">{org.description}</p>
					{/if}
				</button>
			{/each}
		</div>
	{/if}
</div>
