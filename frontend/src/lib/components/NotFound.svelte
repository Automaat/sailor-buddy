<script lang="ts">
	import Compass from '@lucide/svelte/icons/compass';

	interface Props {
		title?: string;
		message?: string;
		backHref?: string;
		backLabel?: string;
	}

	let {
		title = 'Nie znaleziono',
		message = 'Nie znaleźliśmy tego, czego szukasz. Mogło zostać usunięte lub nie masz do niego dostępu.',
		backHref = '/',
		backLabel = 'Wróć na start'
	}: Props = $props();
</script>

<div class="mx-auto max-w-md py-16 text-center">
	<Compass class="mx-auto h-14 w-14 text-[var(--text-muted)]" />
	<h1 class="mt-4 text-2xl font-bold text-[var(--navy)]">{title}</h1>
	<p class="mt-2 text-sm text-[var(--text-muted)]">{message}</p>
	<a
		href={backHref}
		class="mt-6 inline-block rounded-lg bg-[var(--ocean)] px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-[var(--ocean-dark)]"
	>
		{backLabel}
	</a>
</div>
