<script lang="ts">
	import '../app.css';
	import { auth } from '#lib/stores/auth.svelte.js';
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

	let { children } = $props();

	type NavItem = {
		href: string;
		label: string;
		icon: typeof Anchor;
		adminOnly?: boolean;
		alsoActiveUnder?: string[];
	};

	const navItems: NavItem[] = [
		{ href: '/', label: 'Pulpit', icon: Anchor },
		{ href: '/cruises', label: 'Rejsy', icon: Sailboat, alsoActiveUnder: ['/trips', '/voyages'] },
		{ href: '/series', label: 'Serie', icon: Repeat },
		{ href: '/yachts', label: 'Jachty', icon: Ship },
		{ href: '/trainings', label: 'Szkolenia', icon: ClipboardList },
		{ href: '/members', label: 'Członkowie', icon: Users, adminOnly: true }
	];

	async function handleLogout() {
		await auth.logout();
		goto('/login');
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
			goto('/login');
		}
	});

	// Onboarding gate: a signed-in user who has not filled in the profile form
	// is sent there first, and returned to the original page once the saved
	// profile marks them onboarded.
	$effect(() => {
		if (!auth.user) return;
		const target = onboardingRedirect(auth.user.onboarded, page.url);
		if (target) goto(target, { replaceState: true });
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
</script>

{#if auth.loading}
	<div class="flex min-h-screen items-center justify-center bg-[var(--navy)]">
		<ClubLogo inverted class="h-14 w-auto" />
	</div>
{:else if page.url.pathname.startsWith('/login') || isEnrollRoute(page.url.pathname) || page.url.pathname === ONBOARDING_PATH}
	{@render children()}
{:else}
	<div class="flex min-h-screen">
		<nav class="flex w-60 flex-col bg-[var(--navy)] text-white">
			<a href="/" class="flex items-center gap-3 border-b border-white/10 p-4">
				<ClubLogo inverted alt="" class="h-9 w-auto" />
				<span class="text-lg font-bold">AKŻ AGH</span>
			</a>

			<div class="mt-4 flex flex-1 flex-col gap-1 px-2">
				{#each navItems as item}
					{#if !item.adminOnly || auth.isAdmin}
						{@const active =
							page.url.pathname === item.href ||
							(item.href !== '/' && page.url.pathname.startsWith(item.href)) ||
							(item.alsoActiveUnder ?? []).some((p) => page.url.pathname.startsWith(p))}
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
