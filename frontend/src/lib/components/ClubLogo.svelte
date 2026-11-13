<script lang="ts">
	let {
		class: className = '',
		inverted = false,
		alt = 'AKŻ AGH'
	}: { class?: string; inverted?: boolean; alt?: string } = $props();
</script>

<img src="/akz-logo.png" {alt} class="{className} {inverted ? 'brightness-0 invert' : ''}" />
