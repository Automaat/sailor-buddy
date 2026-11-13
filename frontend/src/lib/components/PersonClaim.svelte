<script lang="ts">
	import {
		listPersonClaimCandidates,
		getMyPersonLinkRequest,
		createPersonLinkRequest,
		cancelPersonLinkRequest
	} from '#lib/api/routes.js';
	import type { PersonClaimCandidate, PersonLinkRequest } from '#lib/api/types.js';
	import { ApiError } from '#lib/api/client.js';
	import { onMount } from 'svelte';

	let request = $state<PersonLinkRequest | null>(null);
	let candidates = $state<PersonClaimCandidate[]>([]);
	let query = $state('');
	let loaded = $state(false);
	let busy = $state(false);
	let error = $state('');

	const pending = $derived(request?.status === 'pending');
	const approved = $derived(request?.status === 'approved');

	let searchRequest = 0;

	async function search() {
		const current = ++searchRequest;
		try {
			const found = await listPersonClaimCandidates(query.trim());
			if (current === searchRequest) candidates = found ?? [];
		} catch (err) {
			console.error('Failed to search people:', err);
			if (current === searchRequest) error = 'Nie udało się wyszukać osób.';
		}
	}

	onMount(async () => {
		try {
			request = await getMyPersonLinkRequest();
			if (request?.status !== 'pending' && request?.status !== 'approved') await search();
		} catch (err) {
			console.error('Failed to load person link request:', err);
			error = 'Nie udało się wczytać prośby o połączenie.';
		} finally {
			loaded = true;
		}
	});

	function requestError(err: unknown): string {
		if (err instanceof ApiError && err.status === 409) {
			return 'Ta osoba ma już konto albo masz już oczekującą prośbę.';
		}
		if (err instanceof ApiError && err.status === 404) {
			return 'Tej osoby nie ma już na liście.';
		}
		return 'Nie udało się wysłać prośby.';
	}

	async function claim(p: PersonClaimCandidate) {
		busy = true;
		error = '';
		try {
			request = await createPersonLinkRequest(p.id);
		} catch (err) {
			error = requestError(err);
		} finally {
			busy = false;
		}
	}

	async function cancel() {
		busy = true;
		error = '';
		try {
			await cancelPersonLinkRequest();
			request = request ? { ...request, status: 'cancelled' } : null;
			await search();
		} catch (err) {
			console.error('Failed to cancel person link request:', err);
			error = 'Nie udało się anulować prośby.';
		} finally {
			busy = false;
		}
	}
</script>

<section class="rounded-lg border border-gray-200 bg-white p-6 shadow-sm" aria-label="Moja historia w klubie">
	<h2 class="mb-2 font-semibold text-[var(--navy)]">Moja historia w klubie</h2>

	{#if !loaded}
		<p class="text-sm text-[var(--text-muted)]">Wczytywanie...</p>
	{:else if pending && request}
		<p class="text-sm" role="status">
			Prośba o połączenie z osobą <strong>{request.person_name}</strong> czeka na zatwierdzenie
			przez zarząd.
		</p>
		<button
			type="button"
			onclick={cancel}
			disabled={busy}
			class="mt-3 rounded-lg border px-4 py-2 text-sm hover:bg-gray-50 disabled:opacity-50"
		>
			Anuluj prośbę
		</button>
	{:else if approved && request}
		<p class="text-sm text-emerald-700" role="status">
			Konto połączone z osobą <strong>{request.person_name}</strong>.
		</p>
	{:else}
		<p class="mb-3 text-sm text-[var(--text-muted)]">
			Pływałeś z nami, zanim założyłeś konto? Znajdź siebie na liście i kliknij „To ja”. Zarząd
			sprawdzi prośbę i połączy dawne rejsy z Twoim kontem. Możesz to zrobić też później w profilu.
		</p>
		{#if request?.status === 'rejected'}
			<p class="mb-3 text-sm text-red-600" role="status">
				Zarząd odrzucił prośbę o połączenie z osobą {request.person_name}.
			</p>
		{/if}
		<label for="claim-search" class="mb-1 block text-sm font-medium text-gray-700">Szukaj osoby</label>
		<input
			id="claim-search"
			type="search"
			bind:value={query}
			oninput={search}
			placeholder="imię lub nazwisko"
			class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
		/>
		{#if candidates.length}
			<ul class="mt-3 divide-y text-sm">
				{#each candidates as p (p.id)}
					<li class="flex items-center justify-between gap-2 py-2">
						<span>
							{p.full_name}
							{#if p.other_names?.length}
								<span class="text-[var(--text-muted)]"> · {p.other_names.join(', ')}</span>
							{/if}
							{#if p.email_match}
								<span class="text-xs text-emerald-700"> · pasuje do Twojego e-maila</span>
							{/if}
						</span>
						<button
							type="button"
							onclick={() => claim(p)}
							disabled={busy}
							aria-label="To ja: {p.full_name}"
							class="rounded-lg border px-3 py-1 text-sm hover:bg-gray-50 disabled:opacity-50"
						>
							To ja
						</button>
					</li>
				{/each}
			</ul>
		{:else if query.trim()}
			<p class="mt-3 text-sm text-[var(--text-muted)]">Nie znaleziono osób bez konta.</p>
		{/if}
	{/if}

	{#if error}
		<p class="mt-3 text-sm text-red-600">{error}</p>
	{/if}
</section>
