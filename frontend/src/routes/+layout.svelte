<script lang="ts">
	import '../app.css';
	import { auth } from '$lib/stores/auth.svelte';
	import { orgStore } from '$lib/stores/org.svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getMe } from '$lib/api/routes';
	import Anchor from '@lucide/svelte/icons/anchor';
	import Sailboat from '@lucide/svelte/icons/sailboat';
	import Ship from '@lucide/svelte/icons/ship';
	import ClipboardList from '@lucide/svelte/icons/clipboard-list';
	import Download from '@lucide/svelte/icons/download';
	import Users from '@lucide/svelte/icons/users';
	import Settings from '@lucide/svelte/icons/settings';
	import Building2 from '@lucide/svelte/icons/building-2';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import ChevronUp from '@lucide/svelte/icons/chevron-up';
	import Plus from '@lucide/svelte/icons/plus';

	let { children } = $props();

	type NavItem = {
		href: string;
		label: string;
		icon: typeof Anchor;
		orgOnly?: boolean;
		adminOnly?: boolean;
	};

	const navItems = $derived<NavItem[]>([
		{ href: '/', label: 'Pulpit', icon: Anchor },
		{ href: '/cruises', label: 'Wydarzenia', icon: Sailboat, orgOnly: true },
		{ href: '/trips', label: 'Planowane', icon: Sailboat },
		{ href: '/voyages', label: 'Zrealizowane', icon: Sailboat },
		{ href: '/yachts', label: 'Jachty', icon: Ship },
		{ href: '/trainings', label: 'Szkolenia', icon: ClipboardList },
		{ href: '/import', label: 'Import', icon: Download },
		{
			href: '/orgs/' + (orgStore.currentSlug ?? '') + '/members',
			label: 'Członkowie',
			icon: Users,
			orgOnly: true,
			adminOnly: true
		},
		{
			href: '/orgs/' + (orgStore.currentSlug ?? '') + '/settings',
			label: 'Ustawienia',
			icon: Settings,
			orgOnly: true,
			adminOnly: true
		}
	]);

	let switcherOpen = $state(false);

	async function handleLogout() {
		orgStore.clear();
		await auth.logout();
		goto('/login');
	}

	function selectOrg(slug: string) {
		orgStore.select(slug);
		switcherOpen = false;
		goto('/');
	}

	// The public enrollment preview shared via invite link. Matched on the
	// exact segment so an unrelated route like /enrollments stays private.
	function isEnrollRoute(pathname: string) {
		return pathname === '/enroll' || pathname.startsWith('/enroll/');
	}

	// Routes a logged-out visitor may reach: the login screen and the
	// enrollment preview.
	function isPublicRoute(pathname: string) {
		return pathname.startsWith('/login') || isEnrollRoute(pathname);
	}

	$effect(() => {
		if (!auth.loading && !auth.isAuthenticated && !isPublicRoute($page.url.pathname)) {
			goto('/login');
		}
	});

	$effect(() => {
		if (auth.isAuthenticated && !auth.user) {
			const currentUid = auth.firebaseUser?.uid;
			let cancelled = false;

			(async () => {
				try {
					const u = await getMe();

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
		<Anchor class="h-10 w-10 text-white" />
	</div>
{:else if $page.url.pathname.startsWith('/login') || $page.url.pathname.startsWith('/join') || isEnrollRoute($page.url.pathname)}
	{@render children()}
{:else}
	<div class="flex min-h-screen">
		<nav class="flex w-60 flex-col bg-[var(--navy)] text-white">
			<div class="flex items-center gap-2 border-b border-white/10 p-4">
				<Anchor class="h-6 w-6" />
				<span class="text-lg font-bold">Sailor Buddy</span>
			</div>

			{#if orgStore.canSwitch}
				<div class="relative border-b border-white/10 p-2">
					<button
						onclick={() => (switcherOpen = !switcherOpen)}
						class="flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm transition-colors hover:bg-white/10"
					>
						<span class="truncate">{orgStore.current?.name ?? 'Wybierz klub'}</span>
						{#if switcherOpen}
							<ChevronUp class="ml-2 h-4 w-4 text-white/50" />
						{:else}
							<ChevronDown class="ml-2 h-4 w-4 text-white/50" />
						{/if}
					</button>
					{#if switcherOpen}
						<div
							class="absolute left-2 right-2 z-10 mt-1 rounded-lg border border-white/10 bg-[var(--navy)] shadow-lg"
						>
							{#each orgStore.orgs as org}
								<button
									onclick={() => selectOrg(org.slug)}
									class="flex w-full items-center gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-white/10 {orgStore.currentSlug ===
									org.slug
										? 'bg-white/15'
										: ''}"
								>
									<Building2 class="h-4 w-4" />
									<span class="truncate">{org.name}</span>
								</button>
							{/each}
							<a
								href="/orgs"
								onclick={() => (switcherOpen = false)}
								class="flex w-full items-center gap-2 border-t border-white/10 px-3 py-2 text-left text-sm text-white/50 transition-colors hover:bg-white/10 hover:text-white"
							>
								<Plus class="h-4 w-4" />
								<span>Zarządzaj klubami</span>
							</a>
						</div>
					{/if}
				</div>
			{:else if orgStore.current}
				<div class="flex items-center gap-2 border-b border-white/10 px-5 py-3 text-sm">
					<Building2 class="h-4 w-4 text-white/50" />
					<span class="truncate">{orgStore.current.name}</span>
				</div>
			{/if}

			<div class="mt-4 flex flex-1 flex-col gap-1 px-2">
				{#each navItems as item}
					{#if (!item.orgOnly || orgStore.isOrgMode) && (!item.adminOnly || orgStore.isOrgAdmin)}
						{@const active =
							$page.url.pathname === item.href ||
							(item.href !== '/' && $page.url.pathname.startsWith(item.href))}
						{@const Icon = item.icon}
						<a
							href={item.href}
							class="flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors hover:bg-white/10 {active
								? 'bg-white/15'
								: ''}"
						>
							<Icon class="h-5 w-5" />
							<span>{item.label}</span>
						</a>
					{/if}
				{/each}
			</div>
			<div class="border-t border-white/10 p-4">
				<a
					href="/profile"
					class="mb-2 block text-sm text-white/70 transition-colors hover:text-white"
				>
					{auth.user?.name || auth.firebaseUser?.displayName || ''}
				</a>
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
