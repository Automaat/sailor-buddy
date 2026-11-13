<script lang="ts">
	import {
		listPersonLinkRequests,
		approvePersonLinkRequest,
		rejectPersonLinkRequest
	} from '#lib/api/routes.js';
	import type { PersonLinkRequest } from '#lib/api/types.js';
	import { ApiError } from '#lib/api/client.js';
	import { linkRequests } from '#lib/stores/linkRequests.svelte.js';
	import { onMount } from 'svelte';

	let { ondecided }: { ondecided?: () => void } = $props();

	let requests = $state<PersonLinkRequest[]>([]);
	let busyID = $state(0);
	let error = $state('');
	let notice = $state('');

	async function load() {
		try {
			requests = (await listPersonLinkRequests()) ?? [];
			linkRequests.pending = requests.length;
		} catch (err) {
			console.error('Failed to load person link requests:', err);
			error = 'Nie udało się wczytać próśb „To ja”.';
		}
	}

	onMount(load);

	function decideError(err: unknown): string {
		if (err instanceof ApiError && err.status === 409) {
			return 'Prośba jest nieaktualna: osoba ma już konto albo prośbę już rozpatrzono.';
		}
		if (err instanceof ApiError && err.status === 422) {
			return 'Nie można połączyć: po połączeniu byłoby za dużo innych nazwisk.';
		}
		return 'Nie udało się zapisać decyzji.';
	}

	async function decide(r: PersonLinkRequest, approve: boolean) {
		busyID = r.id;
		error = '';
		notice = '';
		try {
			if (approve) {
				await approvePersonLinkRequest(r.id);
				notice = `Połączono konto ${r.user_name} z osobą ${r.person_name}.`;
			} else {
				await rejectPersonLinkRequest(r.id);
				notice = `Odrzucono prośbę ${r.user_name}.`;
			}
			ondecided?.();
		} catch (err) {
			error = decideError(err);
		} finally {
			busyID = 0;
			await load();
		}
	}
</script>

{#if requests.length || error || notice}
	<section class="mb-6 rounded-2xl bg-white p-6 shadow-sm" aria-label="Prośby „To ja”">
		<h2 class="mb-3 text-lg font-semibold text-[var(--navy)]">Prośby „To ja”</h2>
		{#if requests.length}
			<ul class="divide-y text-sm">
				{#each requests as r (r.id)}
					<li class="flex items-center justify-between gap-3 py-2 max-md:flex-wrap" data-testid="link-request">
						<span>
							<strong>{r.user_name}</strong>
							<span class="text-[var(--text-muted)]">({r.user_email})</span>
							→
							<a href="/crew/{r.person_id}" class="text-[var(--ocean)] hover:underline">{r.person_name}</a>
							{#if r.person_email}<span class="text-[var(--text-muted)]"> · {r.person_email}</span>{/if}
						</span>
						<span class="flex gap-2">
							<button
								type="button"
								onclick={() => decide(r, true)}
								disabled={busyID !== 0}
								aria-label="Zatwierdź: {r.user_name} → {r.person_name}"
								class="rounded-lg bg-[var(--ocean)] px-3 py-1 text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
							>
								Zatwierdź
							</button>
							<button
								type="button"
								onclick={() => decide(r, false)}
								disabled={busyID !== 0}
								aria-label="Odrzuć: {r.user_name} → {r.person_name}"
								class="rounded-lg border px-3 py-1 hover:bg-gray-50 disabled:opacity-50"
							>
								Odrzuć
							</button>
						</span>
					</li>
				{/each}
			</ul>
		{/if}
		{#if notice}
			<p class="mt-3 text-sm text-emerald-700" role="status">{notice}</p>
		{/if}
		{#if error}
			<p class="mt-3 text-sm text-red-600">{error}</p>
		{/if}
	</section>
{/if}
