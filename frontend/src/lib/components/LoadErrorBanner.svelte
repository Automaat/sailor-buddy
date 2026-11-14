<script lang="ts">
	interface Props {
		/** Labels of the page sections whose request failed, from settle + noteFailure. */
		errors: string[];
	}

	let { errors }: Props = $props();
</script>

{#if errors.length > 0}
	<div role="alert" class="mb-4 rounded-lg bg-red-50 px-4 py-3 text-sm text-red-700">
		Nie udało się wczytać: {errors.join(', ')}. Odśwież stronę, aby spróbować ponownie.
	</div>
{/if}
