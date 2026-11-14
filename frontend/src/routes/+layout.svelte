<script lang="ts">
	import '../app.css';
	import { auth } from '#lib/stores/auth.svelte.js';
	import { linkRequests } from '#lib/stores/linkRequests.svelte.js';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { getMe } from '#lib/api/routes.js';
	import { ONBOARDING_PATH, onboardingRedirect } from '#lib/onboarding.js';
	import Anchor from '@lucide/svelte/icons/anchor';
	import ClubLogo from '#lib/components/ClubLogo.svelte';
	import Sailboat from '@lucide/svelte/icons/sailboat';
	import Ship from '@lucide/svelte/icons/ship';
	import ClipboardList from '@lucide/svelte/icons/clipboard-list';
	import Users from '@lucide/svelte/icons/users';
	import Repeat from '@lucide/svelte/icons/repeat';
	import Menu from '@lucide/svelte/icons/menu';
	import X from '@lucide/svelte/icons/x';
	import Contact from '@lucide/svelte/icons/contact';

	let { children } = $props();

	type NavItem = {
		href: string;
		label: string;
		icon: typeof Anchor;
		adminOnly?: boolean;
		alsoActiveUnder?: string[];
		badge?: () => number;
	};

	const navItems: NavItem[] = [
		{ href: '/', label: 'Pulpit', icon: Anchor },
		{ href: '/cruises', label: 'Rejsy', icon: Sailboat, alsoActiveUnder: ['/trips', '/voyages'] },
		{ href: '/series', label: 'Serie', icon: Repeat },
		{ href: '/yachts', label: 'Jachty', icon: Ship },
		{ href: '/trainings', label: 'Moje kursy', icon: ClipboardList },
		{ href: '/members', label: 'Członkowie', icon: Users, adminOnly: true },
		{ href: '/crew', label: 'Osoby', icon: Contact, adminOnly: true, badge: () => linkRequests.pending }
	];

	// Phone navigation drawer (below md). It closes on every navigation so a
	// tapped link lands on the page, not behind the open menu.
	let menuOpen = $state(false);

	$effect(() => {
		void page.url.pathname;
		menuOpen = false;
	});

	$effect(() => {
		void page.url.pathname;
		if (auth.isAdmin) void linkRequests.refresh();
	});

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') menuOpen = false;
	}

	async function handleLogout() {
		await auth.logout();
		void goto('/login');
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
		if (!auth.loading && !auth.isAuthenticated && !isPublicRoute(page.url.pathname)) {
			void goto('/login');
		}
	});

	// Onboarding gate: a signed-in user who has not filled in the profile form
	// is sent there first, and returned to the original page once the saved
	// profile marks them onboarded.
	$effect(() => {
		if (!auth.user) return;
		const target = onboardingRedirect(auth.user.onboarded, page.url);
		if (target) void goto(target, { replaceState: true });
	});

	$effect(() => {
		if (auth.isAuthenticated && !auth.user) {
			const currentUid = auth.firebaseUser?.uid;
			let cancelled = false;

			void (async () => {
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
</script>

<svelte:window onkeydown={handleKeydown} />

{#if auth.loading}
	<div class="flex min-h-screen items-center justify-center bg-[var(--navy)]">
		<ClubLogo inverted class="h-14 w-auto" />
	</div>
{:else if page.url.pathname.startsWith('/login') || isEnrollRoute(page.url.pathname) || page.url.pathname === ONBOARDING_PATH}
	{@render children()}
{:else}
	{#snippet navLinks(itemClass: string)}
		{#each navItems as item (item.href)}
			{#if !item.adminOnly || auth.isAdmin}
				{@const active =
					page.url.pathname === item.href ||
					(item.href !== '/' && page.url.pathname.startsWith(item.href)) ||
					(item.alsoActiveUnder ?? []).some((p) => page.url.pathname.startsWith(p))}
				{@const Icon = item.icon}
				<a
					href={item.href}
					aria-current={active ? 'page' : undefined}
					class="flex items-center gap-3 rounded-lg px-3 text-sm transition-colors hover:bg-white/10 {itemClass} {active
						? 'bg-white/15'
						: ''}"
				>
					<Icon class="h-5 w-5" />
					<span>{item.label}</span>
					{#if item.badge?.()}
						<span
							class="ml-auto rounded-full bg-amber-400 px-2 text-xs font-semibold text-[var(--navy)]"
							title="Prośby „To ja” czekające na decyzję"
							aria-label="{item.badge()} oczekujących próśb"
						>
							{item.badge()}
						</span>
					{/if}
				</a>
			{/if}
		{/each}
	{/snippet}

	<div class="flex min-h-screen flex-col md:flex-row">
		<header
			class="sticky top-0 z-30 flex h-14 items-center justify-between bg-[var(--navy)] px-2 text-white md:hidden"
		>
			<a href="/" class="flex min-h-11 items-center gap-2 px-2">
				<ClubLogo inverted alt="" class="h-8 w-auto" />
				<span class="font-bold">AKŻ AGH</span>
			</a>
			<button
				type="button"
				onclick={() => (menuOpen = !menuOpen)}
				aria-expanded={menuOpen}
				aria-controls="mobile-menu"
				aria-label={menuOpen ? 'Zamknij menu' : 'Otwórz menu'}
				class="flex h-11 w-11 items-center justify-center rounded-lg hover:bg-white/10"
			>
				{#if menuOpen}
					<X class="h-6 w-6" />
				{:else}
					<Menu class="h-6 w-6" />
				{/if}
			</button>
		</header>

		{#if menuOpen}
			<div class="fixed inset-0 top-14 z-20 md:hidden">
				<button
					type="button"
					tabindex="-1"
					aria-label="Zamknij menu"
					onclick={() => (menuOpen = false)}
					class="absolute inset-0 h-full w-full bg-black/40"
				></button>
				<nav
					id="mobile-menu"
					aria-label="Menu główne"
					class="relative flex max-h-full flex-col overflow-y-auto bg-[var(--navy)] px-2 pb-4 text-white shadow-lg"
				>
					<div class="flex flex-col gap-1 pt-2">
						{@render navLinks('min-h-11 py-2')}
					</div>
					<div class="mt-2 flex flex-col border-t border-white/10 pt-2">
						<a
							href="/profile"
							class="flex min-h-11 items-center rounded-lg px-3 text-sm text-white/80 hover:bg-white/10 hover:text-white"
						>
							{auth.user?.name || auth.firebaseUser?.displayName || 'Profil'}
						</a>
						<button
							type="button"
							onclick={handleLogout}
							class="flex min-h-11 items-center rounded-lg px-3 text-left text-sm text-white/60 hover:bg-white/10 hover:text-white"
						>
							Wyloguj
						</button>
					</div>
				</nav>
			</div>
		{/if}

		<nav class="hidden w-60 shrink-0 flex-col bg-[var(--navy)] text-white md:flex">
			<a href="/" class="flex items-center gap-3 border-b border-white/10 p-4">
				<ClubLogo inverted alt="" class="h-9 w-auto" />
				<span class="text-lg font-bold">AKŻ AGH</span>
			</a>

			<div class="mt-4 flex flex-1 flex-col gap-1 px-2">
				{@render navLinks('py-2')}
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
		<main class="min-w-0 flex-1 overflow-auto p-4 md:p-8">
			{@render children()}
		</main>
	</div>
{/if}
