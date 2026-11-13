<script lang="ts">
	let { class: className = '', inverted = false }: { class?: string; inverted?: boolean } =
		$props();
</script>

<img
	src="/akz-logo.png"
	alt="AKŻ AGH"
	class="{className} {inverted ? 'brightness-0 invert' : ''}"
/>
