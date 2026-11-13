<script lang="ts">
	import Anchor from '@lucide/svelte/icons/anchor';
	import { updateMe } from '#lib/api/routes.js';
	import { errorMessage } from '#lib/api/errors.js';
	import ProfileFields from '#lib/components/ProfileFields.svelte';
	import { profileBody, profileFormFromUser, type ProfileForm } from '#lib/profile.js';
	import { auth } from '#lib/stores/auth.svelte.js';

	let form = $state<ProfileForm>(profileFormFromUser(null, auth.firebaseUser?.displayName ?? ''));
	let seeded = $state(false);
	let saving = $state(false);
	let error = $state('');

	$effect(() => {
		if (!seeded && auth.user) {
			form = profileFormFromUser(auth.user, auth.firebaseUser?.displayName ?? '');
			seeded = true;
		}
	});

	// Saving marks the user onboarded; the layout gate then navigates back to
	// the page that sent them here.
	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = '';
		saving = true;
		try {
			auth.user = await updateMe(profileBody(form));
		} catch (err) {
			error = errorMessage(err);
		} finally {
			saving = false;
		}
	}
</script>

<div class="flex min-h-screen items-center justify-center bg-[var(--surface)] px-4 py-12">
	<div class="w-full max-w-lg rounded-2xl bg-white p-8 shadow-sm">
		<div class="mb-6 flex items-center gap-2 text-[var(--navy)]">
			<Anchor class="h-6 w-6" />
			<h1 class="text-2xl font-bold">Witaj na pokładzie!</h1>
		</div>
		<p class="mb-6 text-sm text-[var(--text-muted)]">
			Uzupełnij dane raz, a zapisy na rejsy zajmą jedno kliknięcie. Imię, nazwisko i patent trafią
			na listę załogi.
		</p>

		{#if error}
			<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
		{/if}

		{#if !auth.user}
			<div class="py-6 text-center text-[var(--text-muted)]">Wczytywanie...</div>
		{:else}
			<form onsubmit={handleSubmit} class="space-y-4">
				<ProfileFields bind:form />
				<button
					type="submit"
					disabled={saving}
					class="w-full rounded-lg bg-[var(--ocean)] px-4 py-2 font-medium text-white transition-colors hover:bg-[var(--ocean-dark)] disabled:opacity-50"
				>
					{saving ? 'Zapisywanie...' : 'Zapisz i przejdź dalej'}
				</button>
			</form>
		{/if}
	</div>
</div>
