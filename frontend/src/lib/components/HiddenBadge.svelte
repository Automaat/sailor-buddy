<script lang="ts">
	interface Props {
		/** Why the record is hidden, shown on hover; defaults to the plain draft reason. */
		reason?: string;
	}

	let { reason = 'Szkic: widzą go tylko zarząd i kadra kapitańska' }: Props = $props();
</script>

<span
	class="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-medium text-amber-800"
	title={reason}
	data-testid="hidden-badge">Ukryte</span
>
