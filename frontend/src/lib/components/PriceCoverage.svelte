<script lang="ts">
	let {
		included,
		notIncluded
	}: {
		included?: readonly string[] | null;
		notIncluded?: readonly string[] | null;
	} = $props();
</script>

{#if included?.length || notIncluded?.length}
	<section class="space-y-3 text-sm" aria-label="Co obejmuje cena">
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
