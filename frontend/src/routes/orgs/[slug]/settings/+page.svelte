<script lang="ts">
	import { api } from '$lib/api/client';
	import { orgStore } from '$lib/stores/org.svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import type { Organization } from '$lib/api/types';

	let slug = $derived((page.params as Record<string, string>).slug);
	let org = $state<Organization | null>(null);
	let loading = $state(true);
	let saving = $state(false);
	let error = $state('');
	let success = $state('');

	let form = $state({ name: '', description: '', city: '', website: '', pzz_club_number: '', logo_url: '' });

	async function load() {
		loading = true;
		try {
			org = await api.get<Organization>(`/orgs/${slug}`);
			form = {
				name: org.name,
				description: org.description ?? '',
				city: org.city ?? '',
				website: org.website ?? '',
				pzz_club_number: org.pzz_club_number ?? '',
				logo_url: org.logo_url ?? ''
			};
		} finally {
			loading = false;
		}
	}

	async function handleSave(e: Event) {
		e.preventDefault();
		error = '';
		success = '';
		saving = true;
		try {
			await api.put(`/orgs/${slug}`, {
				name: form.name,
				description: form.description || undefined,
				city: form.city || undefined,
				website: form.website || undefined,
				pzz_club_number: form.pzz_club_number || undefined,
				logo_url: form.logo_url || undefined
			});
			await orgStore.refresh();
			success = 'Zapisano';
		} catch (e: any) {
			error = e.message;
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!confirm('Czy na pewno chcesz usunąć ten klub? Tej operacji nie można cofnąć.')) return;
		try {
			await api.del(`/orgs/${slug}`);
			orgStore.select(null);
			await orgStore.refresh();
			goto('/orgs');
		} catch (e: any) {
			error = e.message;
		}
	}

	$effect(() => {
		slug;
		load();
	});
</script>

<div class="mx-auto max-w-2xl">
	<h1 class="mb-6 text-2xl font-bold text-[var(--navy)]">Ustawienia klubu</h1>

	{#if loading}
		<p class="text-gray-500">Ładowanie...</p>
	{:else if org}
		{#if error}
			<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
		{/if}
		{#if success}
			<div class="mb-4 rounded-lg bg-green-50 p-3 text-sm text-green-600">{success}</div>
		{/if}

		<div class="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
			<form onsubmit={handleSave} class="space-y-4">
				<div>
					<label class="mb-1 block text-sm font-medium text-gray-700" for="name">Nazwa *</label>
					<input
						id="name"
						type="text"
						bind:value={form.name}
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
						required
					/>
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium text-gray-700" for="slug-display"
						>Slug</label
					>
					<input
						id="slug-display"
						type="text"
						value={slug}
						disabled
						class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-500"
					/>
				</div>
				<div class="grid grid-cols-2 gap-4">
					<div>
						<label class="mb-1 block text-sm font-medium text-gray-700" for="city"
							>Miasto</label
						>
						<input
							id="city"
							type="text"
							bind:value={form.city}
							class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
						/>
					</div>
					<div>
						<label class="mb-1 block text-sm font-medium text-gray-700" for="pzz"
							>Nr klubu PZŻ</label
						>
						<input
							id="pzz"
							type="text"
							bind:value={form.pzz_club_number}
							class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
						/>
					</div>
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium text-gray-700" for="website"
						>Strona WWW</label
					>
					<input
						id="website"
						type="text"
						bind:value={form.website}
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
					/>
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium text-gray-700" for="desc">Opis</label>
					<textarea
						id="desc"
						bind:value={form.description}
						rows="3"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
					></textarea>
				</div>
				<div class="flex items-center justify-between">
					<button
						type="submit"
						disabled={saving}
						class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-white transition-colors hover:bg-[var(--ocean)]/80 disabled:opacity-50"
					>
						{saving ? 'Zapisywanie...' : 'Zapisz'}
					</button>
					<button
						type="button"
						onclick={handleDelete}
						class="rounded-lg bg-red-600 px-4 py-2 text-sm text-white transition-colors hover:bg-red-700"
					>
						Usuń klub
					</button>
				</div>
			</form>
		</div>
	{/if}
</div>
