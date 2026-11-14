<script lang="ts">
	import { auth } from '#lib/stores/auth.svelte.js';
	import { firebaseAuth } from '#lib/firebase.js';
	import { sendEmailVerification } from 'firebase/auth';
	import { getMe } from '#lib/api/routes.js';
	import { goto } from '$app/navigation';
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
				if (auth.user.approved && auth.user.email_verified) void goto('/');
				else message = 'Dostęp nie jest jeszcze aktywny.';
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
	<h1 class="text-2xl font-bold">Dostęp do klubu</h1>
	{#if !auth.user?.email_verified}<p>
			Potwierdź swój adres e-mail, aby uzyskać dostęp do danych klubu.
		</p>
		<button class="rounded-lg border p-2" disabled={busy} onclick={() => act(true)}
			>Wyślij link weryfikacyjny</button
		>{/if}
	{#if !auth.user?.approved}<p>Twoje konto czeka na zatwierdzenie przez administratora.</p>{/if}
	<p>Możesz uzupełnić profil i zapisać się przez otrzymany link do rejsu.</p>
	<a class="block underline" href="/profile">Mój profil</a>
	<button class="rounded-lg border p-2" disabled={busy} onclick={() => act(false)}
		>Sprawdź dostęp</button
	>
	<button class="rounded-lg border p-2" onclick={logout}>Wyloguj się</button>
	{#if message}<p role="status">{message}</p>{/if}
</div>
