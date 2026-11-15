<script lang="ts">
	import {
		formatPrice,
		hasPriceInfo,
		priceLines,
		yourPrice,
		type Membership,
		type Priced
	} from '#lib/pricing.js';

	let {
		priced,
		membership
	}: {
		priced: Priced;
		membership: Membership | null | undefined;
	} = $props();

	const price = $derived(yourPrice(priced, membership));
	const lines = $derived(priceLines(priced));
	const shown = $derived(hasPriceInfo(priced));
</script>

{#if shown}
	<section class="space-y-3 text-sm" aria-label="Cena">
		{#if price !== null}
			<p data-testid="your-price">
				<span class="text-[var(--text-muted)]">Twoja cena:</span>
				<span class="text-lg font-semibold text-[var(--navy)]"
					>{formatPrice(price, priced.currency)}</span
				>
				<span class="text-[var(--text-muted)]">/ os.</span>
			</p>
		{/if}
		{#if lines.length > 0}
			<dl class="grid grid-cols-1 sm:grid-cols-2 sm:gap-1 max-sm:[&>dd]:mb-2">
				{#each lines as line (line.label)}
					<dt class="text-[var(--text-muted)]">{line.label}</dt>
					<dd>{line.value}</dd>
				{/each}
			</dl>
		{/if}
	</section>
{/if}
