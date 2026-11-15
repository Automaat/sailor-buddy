<script lang="ts">
	let {
		travelNotes = $bindable(''),
		minAge = $bindable<number | null>(null)
	}: { travelNotes?: string; minAge?: number | null } = $props();
</script>

<div>
	<label for="min_age" class="mb-1 block text-sm font-medium">Minimalny wiek</label>
	<input
		id="min_age"
		type="number"
		bind:value={minAge}
		min="0"
		max="99"
		step="1"
		placeholder="np. 18"
		class="w-full rounded-lg border px-3 py-2"
	/>
</div>
<div class="sm:col-span-2">
	<label for="travel_notes" class="mb-1 block text-sm font-medium">Dojazd</label>
	<textarea
		id="travel_notes"
		bind:value={travelNotes}
		rows="3"
		maxlength="5000"
		placeholder="np. tanie loty do Katanii, transport z lotniska do portu"
		class="w-full rounded-lg border px-3 py-2"></textarea>
</div>
