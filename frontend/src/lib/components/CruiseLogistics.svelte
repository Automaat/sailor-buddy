<script lang="ts">
	import { isHttpUrl } from '#lib/cruise.js';

	let {
		groupChatUrl,
		travelNotes,
		minAge,
		class: className = ''
	}: {
		groupChatUrl?: string | null;
		travelNotes?: string | null;
		minAge?: number | null;
		class?: string;
	} = $props();

	const chatHref = $derived(groupChatUrl && isHttpUrl(groupChatUrl) ? groupChatUrl : null);
	const hasAny = $derived(!!chatHref || !!travelNotes || minAge != null);
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
				<p class="whitespace-pre-wrap">{travelNotes}</p>
			</div>
		{/if}
		{#if chatHref}
			<p>
				<a
					href={chatHref}
					target="_blank"
					rel="noopener noreferrer"
					class="font-medium text-[var(--ocean)] hover:underline"
				>
					Czat grupy
				</a>
			</p>
		{/if}
	</div>
{/if}
