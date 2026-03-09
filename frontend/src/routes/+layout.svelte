<script lang="ts">
	import '../app.css';
	import { auth } from '$lib/stores/auth.svelte';
	import { orgStore } from '$lib/stores/org.svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { api } from '$lib/api/client';
	import type { User } from '$lib/api/types';

	let { children } = $props();

	const navItems = [
		{ href: '/', label: 'Pulpit', icon: '⚓' },
		{ href: '/cruises', label: 'Rejsy', icon: '⛵' },
		{ href: '/crew', label: 'Załoga', icon: '👥' },
		{ href: '/yachts', label: 'Jachty', icon: '🚢' },
		{ href: '/trainings', label: 'Szkolenia', icon: '📋' },
		{ href: '/import', label: 'Import', icon: '📥' }
	];

	const orgNavItems = $derived([
		{ href: '/', label: 'Pulpit', icon: '⚓' },
		{ href: '/cruises', label: 'Rejsy', icon: '⛵' },
		{ href: '/crew', label: 'Załoga', icon: '👥' },
		{ href: '/yachts', label: 'Jachty', icon: '🚢' },
		{ href: '/orgs/' + (orgStore.currentSlug ?? '') + '/members', label: 'Członkowie', icon: '👤' },
		{ href: '/orgs/' + (orgStore.currentSlug ?? '') + '/settings', label: 'Ustawienia', icon: '⚙️' }
	]);

	let switcherOpen = $state(false);

	async function handleLogout() {
		orgStore.clear();
		await auth.logout();
		goto('/login');
	}

	function selectOrg(slug: string | null) {
		orgStore.select(slug);
		switcherOpen = false;
		goto('/');
	}

	$effect(() => {
		if (!auth.loading && !auth.isAuthenticated && !$page.url.pathname.startsWith('/login')) {
			goto('/login');
		}
	});

	$effect(() => {
		if (auth.isAuthenticated && !auth.user) {
			const currentUid = auth.firebaseUser?.uid;
			let cancelled = false;

			(async () => {
				try {
					const u = await api.get<User>('/auth/me');

					if (
						!cancelled &&
						auth.isAuthenticated &&
						auth.firebaseUser?.uid === currentUid
					) {
						auth.user = u;
					}
				} catch (err) {
					console.error('Failed to fetch authenticated user via /auth/me', err);
				}
			})();

			return () => {
				cancelled = true;
			};
		}
	});

	$effect(() => {
		if (auth.isAuthenticated && auth.user) {
			orgStore.refresh();
		}
	});
</script>

{#if auth.loading}
	<div class="flex min-h-screen items-center justify-center bg-[var(--navy)]">
		<span class="text-4xl">⚓</span>
	</div>
{:else if $page.url.pathname.startsWith('/login') || $page.url.pathname.startsWith('/join')}
	{@render children()}
{:else}
	<div class="flex min-h-screen">
		<nav class="flex w-60 flex-col bg-[var(--navy)] text-white">
			<div class="flex items-center gap-2 border-b border-white/10 p-4">
				<span class="text-2xl">⚓</span>
				<span class="text-lg font-bold">Sailor Buddy</span>
			</div>

			<div class="relative border-b border-white/10 p-2">
				<button
					onclick={() => (switcherOpen = !switcherOpen)}
					class="flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors hover:bg-white/10"
				>
					<span class="truncate">
						{#if orgStore.current}
							{orgStore.current.name}
						{:else}
							Osobisty
						{/if}
					</span>
					<span class="ml-2 text-xs text-white/50">{switcherOpen ? '▲' : '▼'}</span>
				</button>
				{#if switcherOpen}
					<div
						class="absolute left-2 right-2 z-10 mt-1 rounded-lg border border-white/10 bg-[var(--navy)] shadow-lg"
					>
						<button
							onclick={() => selectOrg(null)}
							class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-white/10 {!orgStore.currentSlug
								? 'bg-white/15'
								: ''}"
						>
							<span>👤</span>
							<span>Osobisty</span>
						</button>
						{#each orgStore.orgs as org}
							<button
								onclick={() => selectOrg(org.slug)}
								class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-white/10 {orgStore.currentSlug ===
								org.slug
									? 'bg-white/15'
									: ''}"
							>
								<span>🏢</span>
								<span class="truncate">{org.name}</span>
							</button>
						{/each}
						<a
							href="/orgs"
							onclick={() => (switcherOpen = false)}
							class="flex w-full items-center gap-2 border-t border-white/10 px-3 py-2 text-left text-sm text-white/50 transition-colors hover:bg-white/10 hover:text-white"
						>
							<span>+</span>
							<span>Zarządzaj klubami</span>
						</a>
					</div>
				{/if}
			</div>

			<div class="mt-4 flex flex-1 flex-col gap-1 px-2">
				{#each orgStore.isOrgMode ? orgNavItems : navItems as item}
					{@const active =
						$page.url.pathname === item.href ||
						(item.href !== '/' && $page.url.pathname.startsWith(item.href))}
					<a
						href={item.href}
						class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors hover:bg-white/10 {active
							? 'bg-white/15'
							: ''}"
					>
						<span>{item.icon}</span>
						<span>{item.label}</span>
					</a>
				{/each}
			</div>
			<div class="border-t border-white/10 p-4">
				<div class="mb-2 text-sm text-white/70">
					{auth.user?.name || auth.firebaseUser?.displayName || ''}
				</div>
				<button
					onclick={handleLogout}
					class="text-sm text-white/50 transition-colors hover:text-white"
				>
					Wyloguj
				</button>
			</div>
		</nav>
		<main class="flex-1 overflow-auto p-8">
			{@render children()}
		</main>
	</div>
{/if}
