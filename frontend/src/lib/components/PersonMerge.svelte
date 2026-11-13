<script lang="ts">
	import { listCrew, previewCrewMerge, mergeCrew } from '#lib/api/routes.js';
	import type { CrewMember, PersonMerge } from '#lib/api/types.js';
	import { ApiError } from '#lib/api/client.js';
	import { defaultKept, matchesPerson } from '#lib/people.js';
	import { plural } from '#lib/plural.js';
	import { onMount } from 'svelte';

	let {
		person,
		onmerged,
		oncancel
	}: {
		person: CrewMember;
		onmerged: (result: PersonMerge, keptName: string) => void;
		oncancel: () => void;
	} = $props();

	let people = $state<CrewMember[]>([]);
	let query = $state('');
	let other = $state<CrewMember | null>(null);
	let keptID = $state(0);
	let preview = $state<PersonMerge | null>(null);
	let busy = $state(false);
	let error = $state('');

	const candidates = $derived(
		people.filter((p) => p.id !== person.id && matchesPerson(p, query)).slice(0, 20)
	);
	const bothAccounts = $derived(!!other && other.has_account && person.has_account);
	const keptName = $derived(keptID === person.id ? person.full_name : (other?.full_name ?? ''));

	onMount(async () => {
		try {
			people = await listCrew();
		} catch (err) {
			console.error('Failed to load people:', err);
			error = 'Nie udało się wczytać listy osób.';
		}
	});

	function mergeError(err: unknown): string {
		if (err instanceof ApiError && err.status === 409) {
			return 'Obie osoby mają konta — nie można ich połączyć.';
		}
		if (err instanceof ApiError && err.status === 422) {
			return 'Nie można połączyć: po połączeniu byłoby za dużo innych nazwisk.';
		}
		return 'Nie udało się połączyć osób.';
	}

	function duplicateID(): number {
		return keptID === person.id ? (other?.id ?? 0) : person.id;
	}

	async function loadPreview() {
		preview = null;
		error = '';
		if (!other || bothAccounts) return;
		busy = true;
		try {
			preview = await previewCrewMerge(keptID, duplicateID());
		} catch (err) {
			error = mergeError(err);
		} finally {
			busy = false;
		}
	}

	function pick(p: CrewMember) {
		other = p;
		keptID = defaultKept(person, p).id;
		loadPreview();
	}

	function setKept(id: number) {
		keptID = id;
		loadPreview();
	}

	async function confirmMerge() {
		busy = true;
		error = '';
		try {
			onmerged(await mergeCrew(keptID, duplicateID()), keptName);
		} catch (err) {
			error = mergeError(err);
		} finally {
			busy = false;
		}
	}

	function reset() {
		other = null;
		preview = null;
		error = '';
	}

	function count(n: number, one: string, few: string, many: string): string {
		return `${n} ${plural(n, one, few, many)}`;
	}
</script>

<section class="mt-6 rounded-2xl bg-white p-6 shadow-sm" aria-label="Połącz osoby">
	<h2 class="mb-3 text-lg font-semibold text-[var(--navy)]">Połącz z inną osobą</h2>

	{#if !other}
		<label for="merge-search" class="mb-1 block text-sm font-medium">Szukaj osoby</label>
		<input
			id="merge-search"
			type="search"
			bind:value={query}
			placeholder="imię, nazwisko lub e-mail"
			class="w-full rounded-lg border px-3 py-2"
		/>
		<ul class="mt-3 divide-y text-sm">
			{#each candidates as p (p.id)}
				<li>
					<button type="button" onclick={() => pick(p)} class="w-full px-2 py-2 text-left hover:bg-gray-50">
						{p.full_name}
						{#if p.email}<span class="text-[var(--text-muted)]"> · {p.email}</span>{/if}
						{#if p.has_account}<span class="text-xs text-emerald-700"> · ma konto</span>{/if}
					</button>
				</li>
			{/each}
		</ul>
	{:else}
		<fieldset class="text-sm">
			<legend class="mb-1 font-medium">Którą osobę zachować?</legend>
			{#each [person, other] as p (p.id)}
				<label class="mr-4 inline-flex items-center gap-2">
					<input
						type="radio"
						name="merge-kept"
						value={p.id}
						checked={keptID === p.id}
						disabled={busy || bothAccounts}
						onchange={() => setKept(p.id)}
					/>
					{p.full_name}{p.has_account ? ' (ma konto)' : ''}
				</label>
			{/each}
		</fieldset>

		{#if bothAccounts}
			<p class="mt-3 text-sm text-red-600">Obie osoby mają konta — nie można ich połączyć.</p>
		{:else if preview}
			<div class="mt-4 rounded-lg bg-gray-50 p-4 text-sm" data-testid="merge-summary">
				<p>
					Zostanie <strong>{keptName}</strong>, a
					<strong>{preview.duplicate.full_name}</strong> zostanie usunięta.
				</p>
				{#if preview.person.full_name !== keptName}
					<p class="mt-2">Imię i nazwisko po połączeniu (z konta): {preview.person.full_name}</p>
				{/if}
				<p class="mt-2 font-medium">Przeniesione do {keptName}:</p>
				<ul class="ml-5 list-disc">
					<li>{count(preview.trip_assignments, 'przypisanie do etapu', 'przypisania do etapów', 'przypisań do etapów')}</li>
					<li>{count(preview.voyage_assignments, 'przypisanie do rejsu', 'przypisania do rejsów', 'przypisań do rejsów')}</li>
					<li>{count(preview.opinions, 'opinia', 'opinie', 'opinii')}</li>
					{#if preview.captained_trips || preview.captained_voyages}
						<li>
							kapitan: {count(preview.captained_trips, 'etap', 'etapy', 'etapów')},
							{count(preview.captained_voyages, 'rejs', 'rejsy', 'rejsów')}
						</li>
					{/if}
				</ul>
				{#if preview.shared_assignments || preview.shared_opinions}
					<p class="mt-2">
						Wspólne wpisy obu osób (zostają wpisy {keptName}):
						{count(preview.shared_assignments, 'przypisanie', 'przypisania', 'przypisań')},
						{count(preview.shared_opinions, 'opinia', 'opinie', 'opinii')}.
					</p>
				{/if}
				{#if preview.person.other_names?.length}
					<p class="mt-2">Inne nazwiska po połączeniu: {preview.person.other_names.join(', ')}</p>
				{/if}
				{#if preview.duplicate.has_account}
					<p class="mt-2">Konto osoby {preview.duplicate.full_name} zostanie przypisane do {keptName}.</p>
				{/if}
			</div>
		{/if}
	{/if}

	{#if error}
		<p class="mt-3 text-sm text-red-600">{error}</p>
	{/if}

	<div class="mt-4 flex gap-2">
		{#if other}
			<button
				type="button"
				onclick={confirmMerge}
				disabled={busy || !preview || bothAccounts}
				class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-sm text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
			>
				{busy ? 'Łączenie...' : 'Połącz'}
			</button>
			<button type="button" onclick={reset} class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">
				Wybierz inną osobę
			</button>
		{/if}
		<button type="button" onclick={oncancel} class="rounded-lg border px-4 py-2 text-sm hover:bg-gray-50">
			Anuluj
		</button>
	</div>
</section>
