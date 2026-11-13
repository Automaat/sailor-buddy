<script lang="ts">
	import type { Snippet } from 'svelte';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';

	let { label = 'Więcej akcji', children }: { label?: string; children: Snippet<[() => void]> } =
		$props();

	let open = $state(false);
	let root = $state<HTMLDivElement>();

	function close() {
		open = false;
	}

	function onWindowClick(e: MouseEvent) {
		if (open && root && !root.contains(e.target as Node)) close();
	}

	function onKeydown(e: KeyboardEvent) {
		if (open && e.key === 'Escape') close();
	}
</script>

<svelte:window onclick={onWindowClick} onkeydown={onKeydown} />

<div class="relative" bind:this={root}>
	<button
		type="button"
		aria-label={label}
		title={label}
		aria-haspopup="menu"
		aria-expanded={open}
		onclick={() => (open = !open)}
		class="rounded-lg border px-3 py-2 text-sm hover:bg-gray-50"
	>
		<Ellipsis class="h-5 w-5" />
	</button>
	{#if open}
		<div
			role="menu"
			aria-label={label}
			class="absolute right-0 z-10 mt-1 min-w-44 rounded-lg border bg-white py-1 shadow-lg"
		>
			{@render children(close)}
		</div>
	{/if}
</div>
