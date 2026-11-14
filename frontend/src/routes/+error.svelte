<script lang="ts">
	import { page } from '$app/state';
	import NotFound from '#lib/components/NotFound.svelte';
</script>

<!-- A page load function throws error(404, message, { title, backHref,
backLabel }) when its record is missing (see found in #lib/api/load.ts);
anything else is a failed load. -->
{#if page.status === 404}
	<NotFound
		title={page.error?.title}
		message={page.error?.message}
		backHref={page.error?.backHref}
		backLabel={page.error?.backLabel}
	/>
{:else}
	<NotFound
		title="Nie udało się wczytać strony"
		message="Coś poszło nie tak. Spróbuj ponownie za chwilę."
	/>
{/if}
