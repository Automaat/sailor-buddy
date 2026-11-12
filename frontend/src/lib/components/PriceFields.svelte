<script lang="ts">
	import { currencies, type PriceForm } from '#lib/pricing.js';

	let {
		price = $bindable(),
		included = $bindable(),
		notIncluded = $bindable()
	}: {
		price: PriceForm;
		included?: string;
		notIncluded?: string;
	} = $props();
</script>

<fieldset class="col-span-2 grid grid-cols-2 gap-4">
	<legend class="mb-2 text-sm font-semibold text-[var(--navy)]">Cena</legend>
	<div>
		<label for="currency" class="mb-1 block text-sm font-medium">Waluta</label>
		<select id="currency" bind:value={price.currency} class="w-full rounded-lg border px-3 py-2">
			{#each currencies as c (c)}
				<option value={c}>{c}</option>
			{/each}
		</select>
	</div>
	<div></div>
	<div>
		<label for="cost_pp" class="mb-1 block text-sm font-medium">Cena dla członków i sympatyków / os.</label>
		<input id="cost_pp" type="number" min="0" step="0.01" bind:value={price.cost_per_person} class="w-full rounded-lg border px-3 py-2" />
	</div>
	<div>
		<label for="cost_pp_nm" class="mb-1 block text-sm font-medium">Cena dla gości / os.</label>
		<input id="cost_pp_nm" type="number" min="0" step="0.01" bind:value={price.cost_per_person_non_member} class="w-full rounded-lg border px-3 py-2" />
	</div>
	<div>
		<label for="yacht_fund" class="mb-1 block text-sm font-medium">Kasa jachtowa / os. (szacunkowo)</label>
		<input id="yacht_fund" type="number" min="0" step="0.01" bind:value={price.yacht_fund} class="w-full rounded-lg border px-3 py-2" />
	</div>
	<div>
		<label for="deposit" class="mb-1 block text-sm font-medium">Kaucja zwrotna / os.</label>
		<input id="deposit" type="number" min="0" step="0.01" bind:value={price.deposit} class="w-full rounded-lg border px-3 py-2" />
	</div>
	{#if included !== undefined}
		<div>
			<label for="included" class="mb-1 block text-sm font-medium">W cenie (jedna pozycja w wierszu)</label>
			<textarea id="included" bind:value={included} rows="4" class="w-full rounded-lg border px-3 py-2"></textarea>
		</div>
	{/if}
	{#if notIncluded !== undefined}
		<div>
			<label for="not_included" class="mb-1 block text-sm font-medium">Poza ceną (jedna pozycja w wierszu)</label>
			<textarea id="not_included" bind:value={notIncluded} rows="4" class="w-full rounded-lg border px-3 py-2"></textarea>
		</div>
	{/if}
</fieldset>
