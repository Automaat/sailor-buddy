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
		membership,
		included,
		notIncluded
	}: {
		priced: Priced;
		membership: Membership | null | undefined;
		included?: readonly string[] | null;
		notIncluded?: readonly string[] | null;
	} = $props();

	const price = $derived(yourPrice(priced, membership));
	const lines = $derived(priceLines(priced));
	const shown = $derived(hasPriceInfo(priced, included, notIncluded));
</script>

{#if shown}
	<section class="space-y-3 text-sm" aria-label="Cena">
		{#if price !== null}
			<p data-testid="your-price">
				<span class="text-[var(--text-muted)]">Twoja cena:</span>
				<span class="text-lg font-semibold text-[var(--navy)]">{formatPrice(price, priced.currency)}</span>
				<span class="text-[var(--text-muted)]">/ os.</span>
			</p>
		{/if}
		{#if lines.length > 0}
			<dl class="grid grid-cols-2 gap-1">
				{#each lines as line (line.label)}
					<dt class="text-[var(--text-muted)]">{line.label}</dt>
					<dd>{line.value}</dd>
				{/each}
			</dl>
		{/if}
		{#if included?.length}
			<div>
				<h3 class="mb-1 font-medium">W cenie</h3>
				<ul class="list-disc pl-5" data-testid="price-included">
					{#each included as item, i (i)}
						<li>{item}</li>
					{/each}
				</ul>
			</div>
		{/if}
		{#if notIncluded?.length}
			<div>
				<h3 class="mb-1 font-medium">Poza ceną</h3>
				<ul class="list-disc pl-5" data-testid="price-not-included">
					{#each notIncluded as item, i (i)}
						<li>{item}</li>
					{/each}
				</ul>
			</div>
		{/if}
	</section>
{/if}
