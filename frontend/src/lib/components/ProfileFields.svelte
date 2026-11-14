<script lang="ts">
	import { PATENT_OPTIONS, type ProfileForm } from '#lib/profile.js';

	let { form = $bindable() }: { form: ProfileForm } = $props();

	const input = 'w-full rounded-lg border border-gray-300 px-3 py-2 text-sm';
	const label = 'mb-1 block text-sm font-medium text-gray-700';
</script>

<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
	<div>
		<label class={label} for="firstName">Imię *</label>
		<input
			id="firstName"
			type="text"
			bind:value={form.firstName}
			required
			maxlength="100"
			autocomplete="given-name"
			class={input}
		/>
	</div>
	<div>
		<label class={label} for="lastName">Nazwisko *</label>
		<input
			id="lastName"
			type="text"
			bind:value={form.lastName}
			required
			maxlength="100"
			autocomplete="family-name"
			class={input}
		/>
	</div>
</div>
<div>
	<label class={label} for="phone">Telefon</label>
	<input
		id="phone"
		type="tel"
		bind:value={form.phone}
		maxlength="30"
		pattern={'\\+?[0-9 ()\\-]{6,29}'}
		title="Numer telefonu, np. +48 600 100 200"
		placeholder="+48 600 100 200"
		autocomplete="tel"
		class={input}
	/>
	<p class="mt-1 text-xs text-[var(--text-muted)]">
		Kapitan zadzwoni przed rejsem, jeśli będzie trzeba.
	</p>
</div>
<div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
	<div>
		<label class={label} for="patentType">Patent żeglarski</label>
		<select id="patentType" bind:value={form.patentType} class={input}>
			{#each PATENT_OPTIONS as opt (opt.value)}
				<option value={opt.value}>{opt.label}</option>
			{/each}
		</select>
	</div>
	<div>
		<label class={label} for="patentNumber">Numer patentu</label>
		<input
			id="patentNumber"
			type="text"
			bind:value={form.patentNumber}
			disabled={!form.patentType}
			required={!!form.patentType}
			class="{input} disabled:bg-gray-50 disabled:text-gray-400"
		/>
	</div>
</div>
