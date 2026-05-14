<script lang="ts">
	import { api } from '$lib/api/client';
	import { auth } from '$lib/stores/auth.svelte';
	import { orgStore } from '$lib/stores/org.svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import type { OrgInviteInfo } from '$lib/api/types';
	import Anchor from '@lucide/svelte/icons/anchor';
	import XCircle from '@lucide/svelte/icons/x-circle';
	import PartyPopper from '@lucide/svelte/icons/party-popper';

	let token = $derived((page.params as Record<string, string>).token);
	let info = $state<OrgInviteInfo | null>(null);
	let loading = $state(true);
	let joining = $state(false);
	let error = $state('');
	let joined = $state(false);

	const roleLabels: Record<string, string> = {
		admin: 'Admin',
		captain: 'Kapitan',
		crew: 'Załogant'
	};

	async function load() {
		loading = true;
		error = '';
		try {
			info = await api.get<OrgInviteInfo>(`/join/${token}`);
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function handleJoin() {
		joining = true;
		error = '';
		try {
			const result = await api.post<{ org_slug: string }>(`/join/${token}`);
			joined = true;
			await orgStore.refresh();
			orgStore.select(result.org_slug);
			setTimeout(() => goto('/'), 1500);
		} catch (e: any) {
			error = e.message;
		} finally {
			joining = false;
		}
	}

	$effect(() => {
		if (auth.isAuthenticated && auth.user) {
			load();
		}
	});
</script>

<div class="flex min-h-screen items-center justify-center bg-[var(--navy)]">
	<div class="w-full max-w-md rounded-lg bg-white p-8 shadow-lg">
		{#if !auth.isAuthenticated}
			<div class="text-center">
				<Anchor class="mx-auto h-10 w-10 text-[var(--navy)]" />
				<h1 class="mt-4 text-xl font-bold text-[var(--navy)]">Zaproszenie do klubu</h1>
				<p class="mt-2 text-gray-500">Zaloguj się, aby dołączyć</p>
				<a
					href="/login"
					class="mt-4 inline-block rounded-lg bg-[var(--ocean)] px-6 py-2 text-white transition-colors hover:bg-[var(--ocean)]/80"
				>
					Zaloguj się
				</a>
			</div>
		{:else if loading}
			<div class="text-center">
				<Anchor class="mx-auto h-10 w-10 animate-pulse text-[var(--navy)]" />
				<p class="mt-4 text-gray-500">Ładowanie...</p>
			</div>
		{:else if error}
			<div class="text-center">
				<XCircle class="mx-auto h-10 w-10 text-red-500" />
				<h1 class="mt-4 text-xl font-bold text-[var(--navy)]">Błąd</h1>
				<p class="mt-2 text-red-600">{error}</p>
				<a
					href="/"
					class="mt-4 inline-block rounded-lg bg-[var(--ocean)] px-6 py-2 text-white transition-colors hover:bg-[var(--ocean)]/80"
				>
					Wróć do aplikacji
				</a>
			</div>
		{:else if joined}
			<div class="text-center">
				<PartyPopper class="mx-auto h-10 w-10 text-[var(--ocean)]" />
				<h1 class="mt-4 text-xl font-bold text-[var(--navy)]">Dołączono!</h1>
				<p class="mt-2 text-gray-500">Przekierowywanie...</p>
			</div>
		{:else if info}
			<div class="text-center">
				<Anchor class="mx-auto h-10 w-10 text-[var(--navy)]" />
				<h1 class="mt-4 text-xl font-bold text-[var(--navy)]">{info.org_name}</h1>
				<p class="mt-2 text-gray-500">
					Zostaniesz dodany jako
					<span class="font-medium text-[var(--ocean)]"
						>{roleLabels[info.role] ?? info.role}</span
					>
				</p>

				{#if info.already_member}
					<div class="mt-4 rounded-lg bg-yellow-50 p-3 text-sm text-yellow-700">
						Jesteś już członkiem tego klubu
					</div>
					<a
						href="/"
						class="mt-4 inline-block rounded-lg bg-[var(--ocean)] px-6 py-2 text-white transition-colors hover:bg-[var(--ocean)]/80"
					>
						Wróć do aplikacji
					</a>
				{:else}
					<button
						onclick={handleJoin}
						disabled={joining}
						class="mt-6 w-full rounded-lg bg-[var(--ocean)] px-6 py-3 text-white transition-colors hover:bg-[var(--ocean)]/80 disabled:opacity-50"
					>
						{joining ? 'Dołączanie...' : 'Dołącz do klubu'}
					</button>
				{/if}
			</div>
		{/if}
	</div>
</div>
