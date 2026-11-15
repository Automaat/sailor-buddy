<script lang="ts">
	let {
		travelNotes,
		minAge,
		class: className = ''
	}: {
		travelNotes?: string | null;
		minAge?: number | null;
		class?: string;
	} = $props();

	const hasAny = $derived(!!travelNotes || minAge != null);
</script>

{#if hasAny}
	<div class="space-y-2 text-sm {className}" data-testid="cruise-logistics">
		{#if minAge != null}
			<p>
				<span class="text-[var(--text-muted)]">Minimalny wiek:</span>
				<span class="font-medium">{minAge}+</span>
			</p>
		{/if}
		{#if travelNotes}
			<div>
				<p class="text-[var(--text-muted)]">Dojazd:</p>
				<p class="whitespace-pre-wrap [overflow-wrap:anywhere]">{travelNotes}</p>
			</div>
		{/if}
	</div>
{/if}
