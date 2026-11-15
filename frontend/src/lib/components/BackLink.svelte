<script lang="ts">
	/** A link to the page's parent, shown above the page title. */
	let { href, label }: { href: string; label: string } = $props();
</script>

<a
	{href}
	class="tap-target inline-flex items-baseline gap-1 text-sm text-[var(--ocean)] hover:underline"
	data-testid="back-link"
>
	<span aria-hidden="true">←</span>
	<span class="min-w-0 [overflow-wrap:anywhere]">{label}</span>
</a>
