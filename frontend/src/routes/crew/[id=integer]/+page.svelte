<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { getCrew, updateCrew, deleteCrew } from '#lib/api/routes.js';
	import type { CrewMember } from '#lib/api/types.js';
	import { ApiError } from '#lib/api/client.js';
	import { parseOtherNames } from '#lib/people.js';
	import NotFound from '#lib/components/NotFound.svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	let person = $state<CrewMember | null>(null);
	let loading = $state(true);
	let notFound = $state(false);
	let editing = $state(false);
	let saving = $state(false);
	let error = $state('');
	let form = $state({ full_name: '', other_names: '', email: '', phone: '', patent_number: '' });

	const id = $derived(Number(page.params.id));

	onMount(async () => {
		try {
			person = await getCrew(id);
		} catch (err) {
			if (err instanceof ApiError && err.status === 404) {
				notFound = true;
			} else {
				console.error('Failed to load person:', err);
			}
		} finally {
			loading = false;
		}
	});

	function startEdit(p: CrewMember) {
		form = {
			full_name: p.full_name,
			other_names: (p.other_names ?? []).join(', '),
			email: p.email ?? '',
			phone: p.phone ?? '',
			patent_number: p.patent_number ?? ''
		};
		error = '';
		editing = true;
	}

	async function handleSave(e: Event) {
		e.preventDefault();
		if (!person) return;
		saving = true;
		error = '';
		const body = {
			full_name: form.full_name,
			other_names: parseOtherNames(form.other_names),
			email: form.email || undefined,
			phone: form.phone || undefined,
			patent_number: form.patent_number || undefined,
			pzz_license_type: person.pzz_license_type,
			pzz_license_number: person.pzz_license_number,
			emergency_contact_name: person.emergency_contact_name,
			emergency_contact_phone: person.emergency_contact_phone
		};
		try {
			await updateCrew(id, body);
			person = await getCrew(id);
			editing = false;
		} catch (err) {
			console.error('Failed to update person:', err);
			error = 'Nie udało się zapisać zmian.';
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!confirm('Usunąć tę osobę?')) return;
		await deleteCrew(id);
		goto('/crew');
	}
</script>

{#if loading}
	<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
{:else if person}
	<div class="mx-auto max-w-2xl">
		<div class="mb-6 flex items-center justify-between max-md:flex-wrap max-md:gap-3">
			<div>
				<h1 class="text-3xl font-bold text-[var(--navy)]">{person.full_name}</h1>
				<span class="mt-1 inline-block rounded-full px-2 py-0.5 text-xs font-medium {person.has_account
					? 'bg-emerald-50 text-emerald-700'
					: 'bg-gray-100 text-gray-600'}">{person.has_account ? 'ma konto' : 'bez konta'}</span>
			</div>
			{#if auth.isAdmin && !editing}
				<div class="flex gap-2">
					<button onclick={() => person && startEdit(person)} class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">
						Edytuj
					</button>
					{#if !person.has_account}
						<button onclick={handleDelete} class="rounded-lg border border-red-200 px-4 py-2 text-sm text-red-600 hover:bg-red-50">
							Usuń
						</button>
					{/if}
				</div>
			{/if}
		</div>

		{#if editing}
			<form onsubmit={handleSave} class="rounded-2xl bg-white p-6 shadow-sm">
				<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
					<div>
						<label for="edit-name" class="mb-1 block text-sm font-medium">Imię i nazwisko *</label>
						<input id="edit-name" type="text" bind:value={form.full_name} required class="w-full rounded-lg border px-3 py-2" />
					</div>
					<div>
						<label for="edit-other-names" class="mb-1 block text-sm font-medium">Inne nazwiska / pisownie</label>
						<input
							id="edit-other-names"
							type="text"
							bind:value={form.other_names}
							placeholder="oddzielone przecinkami"
							class="w-full rounded-lg border px-3 py-2"
						/>
					</div>
					<div>
						<label for="edit-email" class="mb-1 block text-sm font-medium">Email</label>
						<input id="edit-email" type="email" bind:value={form.email} class="w-full rounded-lg border px-3 py-2" />
					</div>
					<div>
						<label for="edit-phone" class="mb-1 block text-sm font-medium">Telefon</label>
						<input id="edit-phone" type="tel" bind:value={form.phone} class="w-full rounded-lg border px-3 py-2" />
					</div>
					<div>
						<label for="edit-patent" class="mb-1 block text-sm font-medium">Numer patentu</label>
						<input id="edit-patent" type="text" bind:value={form.patent_number} class="w-full rounded-lg border px-3 py-2" />
					</div>
				</div>
				{#if person.has_account}
					<p class="mt-3 text-xs text-[var(--text-muted)]">
						Imię, nazwisko, telefon i patent nadpisze właściciel konta, gdy zapisze swój profil.
					</p>
				{/if}
				{#if error}
					<p class="mt-3 text-sm text-red-600">{error}</p>
				{/if}
				<div class="mt-4 flex gap-2">
					<button type="submit" disabled={saving} class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50">
						{saving ? 'Zapisywanie...' : 'Zapisz'}
					</button>
					<button type="button" onclick={() => (editing = false)} class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">
						Anuluj
					</button>
				</div>
			</form>
		{:else}
			<div class="rounded-2xl bg-white p-6 shadow-sm">
				<dl class="grid grid-cols-1 text-sm sm:grid-cols-2 sm:gap-4 max-sm:[&>dd]:mb-3">
					{#if person.other_names?.length}
						<dt class="text-[var(--text-muted)]">Inne nazwiska</dt>
						<dd>{person.other_names.join(', ')}</dd>
					{/if}
					{#if person.email}
						<dt class="text-[var(--text-muted)]">Email</dt>
						<dd>{person.email}</dd>
					{/if}
					{#if person.phone}
						<dt class="text-[var(--text-muted)]">Telefon</dt>
						<dd>{person.phone}</dd>
					{/if}
					{#if person.patent_number}
						<dt class="text-[var(--text-muted)]">Numer patentu</dt>
						<dd>{person.patent_number}</dd>
					{/if}
				</dl>
			</div>
		{/if}
	</div>
{:else if notFound}
	<NotFound
		title="Nie znaleziono osoby"
		message="Ta osoba nie istnieje lub nie masz do niej dostępu."
		backHref="/crew"
		backLabel="Wszystkie osoby"
	/>
{:else}
	<NotFound
		title="Nie udało się wczytać osoby"
		message="Coś poszło nie tak. Spróbuj ponownie za chwilę."
		backHref="/crew"
		backLabel="Wszystkie osoby"
	/>
{/if}
