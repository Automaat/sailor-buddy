<script lang="ts">
	let {
		included = $bindable(''),
		notIncluded = $bindable('')
	}: { included?: string; notIncluded?: string } = $props();
</script>

<fieldset class="grid grid-cols-1 gap-4 sm:col-span-2 sm:grid-cols-2">
	<legend class="mb-2 text-sm font-semibold text-[var(--navy)]">Co obejmuje cena etapu</legend>
	<div>
		<label for="included" class="mb-1 block text-sm font-medium"
			>W cenie (jedna pozycja w wierszu)</label
		>
		<textarea
			id="included"
			bind:value={included}
			rows="4"
			class="w-full rounded-lg border px-3 py-2"></textarea>
	</div>
	<div>
		<label for="not_included" class="mb-1 block text-sm font-medium"
			>Poza ceną (jedna pozycja w wierszu)</label
		>
		<textarea
			id="not_included"
			bind:value={notIncluded}
			rows="4"
			class="w-full rounded-lg border px-3 py-2"></textarea>
	</div>
</fieldset>
