<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { listCrew, createCrew } from '#lib/api/routes.js';
	import type { CrewMember } from '#lib/api/types.js';
	import {
		accountFilters,
		matchesAccount,
		matchesPerson,
		parseOtherNames,
		type AccountFilter
	} from '#lib/people.js';
	import Users from '@lucide/svelte/icons/users';
	import PersonLinkRequests from '#lib/components/PersonLinkRequests.svelte';

	let people = $state<CrewMember[]>([]);
	let loading = $state(true);
	let showForm = $state(false);
	let form = $state({ full_name: '', other_names: '', email: '', patent_number: '' });
	let saving = $state(false);
	let query = $state('');
	let account = $state<AccountFilter>('all');

	const visible = $derived(
		people.filter((p) => matchesAccount(p, account) && matchesPerson(p, query))
	);

	async function load() {
		loading = true;
		try {
			people = await listCrew();
		} catch (err) {
			console.error('Failed to load people:', err);
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		void load();
	});

	async function handleAdd(e: Event) {
		e.preventDefault();
		saving = true;
		try {
			const person = await createCrew({
				full_name: form.full_name,
				other_names: parseOtherNames(form.other_names),
				email: form.email || undefined,
				patent_number: form.patent_number || undefined
			});
			people = [...people, person].sort((a, b) => a.full_name.localeCompare(b.full_name, 'pl'));
			form = { full_name: '', other_names: '', email: '', patent_number: '' };
			showForm = false;
		} catch (err) {
			console.error('Failed to add person:', err);
		} finally {
			saving = false;
		}
	}
</script>

<div>
	<div class="mb-6 flex items-center justify-between max-md:flex-wrap max-md:gap-3">
		<h1 class="text-3xl font-bold text-[var(--navy)]">Osoby</h1>
		{#if auth.isAdmin}
			<button
				onclick={() => (showForm = !showForm)}
				class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm font-medium text-white hover:bg-[var(--ocean-dark)]"
			>
				{showForm ? 'Anuluj' : '+ Dodaj osobę'}
			</button>
		{/if}
	</div>

	{#if auth.isAdmin}
		<PersonLinkRequests ondecided={load} />
	{/if}

	{#if showForm && auth.isAdmin}
		<form onsubmit={handleAdd} class="mb-6 rounded-2xl bg-white p-6 shadow-sm">
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
				<div>
					<label for="name" class="mb-1 block text-sm font-medium">Imię i nazwisko *</label>
					<input
						id="name"
						type="text"
						bind:value={form.full_name}
						required
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="other-names" class="mb-1 block text-sm font-medium"
						>Inne nazwiska / pisownie</label
					>
					<input
						id="other-names"
						type="text"
						bind:value={form.other_names}
						placeholder="oddzielone przecinkami"
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="email" class="mb-1 block text-sm font-medium">Email</label>
					<input
						id="email"
						type="email"
						bind:value={form.email}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
				<div>
					<label for="patent" class="mb-1 block text-sm font-medium">Numer patentu</label>
					<input
						id="patent"
						type="text"
						bind:value={form.patent_number}
						class="w-full rounded-lg border px-3 py-2"
					/>
				</div>
			</div>
			<button
				type="submit"
				disabled={saving}
				class="mt-4 rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
			>
				{saving ? 'Dodawanie...' : 'Dodaj'}
			</button>
		</form>
	{/if}

	<div class="mb-4 flex flex-wrap items-center gap-3">
		<input
			type="search"
			bind:value={query}
			placeholder="Szukaj po nazwisku, innym nazwisku lub e-mailu"
			aria-label="Szukaj osób"
			class="min-w-0 flex-1 rounded-lg border px-3 py-2"
		/>
		<div class="flex gap-1" role="group" aria-label="Konto">
			{#each accountFilters as f (f.value)}
				<button
					type="button"
					onclick={() => (account = f.value)}
					aria-pressed={account === f.value}
					class="rounded-full px-3 py-1 text-sm {account === f.value
						? 'bg-[var(--ocean)] text-white'
						: 'bg-white text-[var(--text-muted)] hover:bg-gray-50'}"
				>
					{f.label}
				</button>
			{/each}
		</div>
	</div>

	{#if loading}
		<div class="py-12 text-center text-[var(--text-muted)]">Wczytywanie...</div>
	{:else if people.length === 0}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<Users class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
			<p class="mt-4 text-lg text-[var(--text-muted)]">Brak osób</p>
		</div>
	{:else if visible.length === 0}
		<p class="py-12 text-center text-[var(--text-muted)]">Nikt nie pasuje do wyszukiwania</p>
	{:else}
		<div class="grid gap-3">
			{#each visible as person (person.id)}
				<a
					href="/crew/{person.id}"
					data-testid="person"
					class="flex min-w-0 items-center justify-between gap-3 rounded-2xl bg-white px-6 py-4 shadow-sm transition-shadow hover:shadow-md"
				>
					<div class="min-w-0">
						<span class="font-semibold text-[var(--navy)]">{person.full_name}</span>
						{#if person.other_names?.length}
							<span class="ml-1 text-sm text-[var(--text-muted)]"
								>({person.other_names.join(', ')})</span
							>
						{/if}
						{#if person.email}
							<span class="ml-2 text-sm break-all text-[var(--text-muted)]">{person.email}</span>
						{/if}
					</div>
					<div class="flex shrink-0 items-center gap-3">
						{#if person.patent_number}
							<span class="text-sm text-[var(--text-muted)]">Patent: {person.patent_number}</span>
						{/if}
						{#if person.has_account}
							<span
								class="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700"
								>ma konto</span
							>
						{:else}
							<span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600"
								>bez konta</span
							>
						{/if}
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>
