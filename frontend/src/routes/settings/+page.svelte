<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
</script>

<div class="mx-auto max-w-2xl">
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Ustawienia</h1>

	<div class="rounded-2xl bg-white p-6 shadow-sm">
		<h2 class="mb-4 font-semibold text-[var(--navy)]">Profil</h2>
		<dl class="grid grid-cols-2 gap-4 text-sm">
			<dt class="text-[var(--text-muted)]">Imię</dt>
			<dd>{auth.user?.name}</dd>
			<dt class="text-[var(--text-muted)]">Email</dt>
			<dd>{auth.user?.email}</dd>
		</dl>
	</div>
</div>
