<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { firebaseAuth } from '#lib/firebase.js';
	import { sendEmailVerification } from 'firebase/auth';
	import { getMe } from '#lib/api/routes.js';
	import { goto } from '$app/navigation';
	import { hasClubAccess } from '#lib/access.js';
	let busy = $state(false);
	let message = $state('');
	async function act(send: boolean) {
		busy = true;
		message = '';
		try {
			const user = firebaseAuth.currentUser;
			if (!user) {
				void goto('/login');
				return;
			}
			if (send) {
				await sendEmailVerification(user);
				message = 'Wysłano link do potwierdzenia adresu e-mail.';
			} else {
				await user.reload();
				await user.getIdToken(true);
				const profile = await getMe();
				if (firebaseAuth.currentUser?.uid !== user.uid) return;
				auth.user = profile;
				if (hasClubAccess(auth.user)) void goto('/');
				else if (auth.user.suspended) message = 'Konto jest zawieszone.';
				else message = 'Adres e-mail nie jest jeszcze potwierdzony.';
			}
		} catch (err) {
			message = err instanceof Error ? err.message : 'Nie udało się sprawdzić konta';
		} finally {
			busy = false;
		}
	}
	async function logout() {
		await auth.logout();
		void goto('/login');
	}
</script>

<div class="mx-auto max-w-lg space-y-4 p-8">
	{#if auth.user?.suspended}
		<h1 class="text-2xl font-bold">Konto zawieszone</h1>
		<p>Twoje konto zostało zawieszone przez klub. Skontaktuj się z zarządem klubu.</p>
	{:else}
		<h1 class="text-2xl font-bold">Potwierdź adres e-mail</h1>
		<p>
			Potwierdź adres e-mail <strong>{auth.user?.email}</strong>, aby uzyskać dostęp do danych
			klubu. Wyślij link weryfikacyjny i otwórz go z poczty.
		</p>
		<button class="rounded-lg border p-2" disabled={busy} onclick={() => act(true)}
			>Wyślij link weryfikacyjny</button
		>
		<p>Do tego czasu możesz uzupełnić profil i zapisać się przez otrzymany link do rejsu.</p>
		<a class="block underline" href="/profile">Mój profil</a>
		<button class="rounded-lg border p-2" disabled={busy} onclick={() => act(false)}
			>Sprawdź dostęp</button
		>
	{/if}
	<button class="rounded-lg border p-2" onclick={logout}>Wyloguj się</button>
	{#if message}<p role="status">{message}</p>{/if}
</div>
