<script lang="ts">
	import '../app.css';
	import { auth } from '$lib/stores/auth.svelte';
	import { goto, afterNavigate } from '$app/navigation';
	import { page } from '$app/stores';

	let { children } = $props();

	const navItems = [
		{ href: '/', label: 'Dashboard', icon: '⚓' },
		{ href: '/cruises', label: 'Cruises', icon: '⛵' },
		{ href: '/crew', label: 'Crew', icon: '👥' },
		{ href: '/yachts', label: 'Yachts', icon: '🚢' },
		{ href: '/trainings', label: 'Trainings', icon: '📋' },
		{ href: '/import', label: 'Import', icon: '📥' }
	];

	function handleLogout() {
		auth.logout();
		goto('/login');
	}

	afterNavigate(() => {
		if (!auth.isAuthenticated && !$page.url.pathname.startsWith('/login')) {
			goto('/login');
		}
	});
</script>

{#if $page.url.pathname.startsWith('/login')}
	{@render children()}
{:else}
	<div class="flex min-h-screen">
		<nav class="flex w-60 flex-col bg-[var(--navy)] text-white">
			<div class="flex items-center gap-2 border-b border-white/10 p-4">
				<span class="text-2xl">⚓</span>
				<span class="text-lg font-bold">Sailor Buddy</span>
			</div>
			<div class="mt-4 flex flex-1 flex-col gap-1 px-2">
				{#each navItems as item}
					{@const active =
						$page.url.pathname === item.href ||
						(item.href !== '/' && $page.url.pathname.startsWith(item.href))}
					<a
						href={item.href}
						onclick={(e) => { e.preventDefault(); goto(item.href); }}
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
				<div class="mb-2 text-sm text-white/70">{auth.user?.name}</div>
				<button
					onclick={handleLogout}
					class="text-sm text-white/50 transition-colors hover:text-white"
				>
					Logout
				</button>
			</div>
		</nav>
		<main class="flex-1 overflow-auto p-8">
			{@render children()}
		</main>
	</div>
{/if}
