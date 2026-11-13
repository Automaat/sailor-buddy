<script lang="ts">
	import {
		updateProfile,
		updatePassword,
		reauthenticateWithCredential,
		EmailAuthProvider
	} from 'firebase/auth';
	import { auth } from '#lib/stores/auth.svelte.js';
	import { updateMe } from '#lib/api/routes.js';
	import { errorMessage } from '#lib/api/errors.js';
	import ProfileFields from '#lib/components/ProfileFields.svelte';
	import PersonClaim from '#lib/components/PersonClaim.svelte';
	import { profileBody, profileFormFromUser, type ProfileForm } from '#lib/profile.js';

	const fbUser = auth.firebaseUser;

	let profileForm = $state<ProfileForm>(profileFormFromUser(auth.user, fbUser?.displayName ?? ''));
	let photoURL = $state(fbUser?.photoURL ?? '');
	let profileSaving = $state(false);
	let profileError = $state('');
	let profileSuccess = $state('');

	// The DB profile (/auth/me) loads asynchronously in the layout. Seed the
	// form once it arrives, before the user edits it.
	let profileLoaded = $state(!!auth.user);
	$effect(() => {
		if (!profileLoaded && auth.user) {
			profileForm = profileFormFromUser(auth.user, fbUser?.displayName ?? '');
			profileLoaded = true;
		}
	});

	const hasPasswordProvider = !!fbUser?.providerData.some(
		(p) => p.providerId === 'password'
	);

	let pwForm = $state({ current: '', next: '', confirm: '' });
	let pwSaving = $state(false);
	let pwError = $state('');
	let pwSuccess = $state('');

	// fbError maps Firebase Auth error codes to Polish messages. The shared
	// errorMessage helper only understands the API error envelope, not the
	// `code`-tagged errors thrown by the Firebase SDK.
	function fbError(e: unknown): string {
		const code = (e as { code?: string }).code;
		switch (code) {
			case 'auth/wrong-password':
			case 'auth/invalid-credential':
				return 'Nieprawidłowe hasło';
			case 'auth/weak-password':
				return 'Hasło zbyt słabe (min. 8 znaków)';
			case 'auth/requires-recent-login':
				return 'Zaloguj się ponownie, aby zmienić hasło';
			case 'auth/too-many-requests':
				return 'Zbyt wiele prób. Spróbuj później';
		}
		return e instanceof Error ? e.message : 'Coś poszło nie tak';
	}

	async function handleProfileSave(e: Event) {
		e.preventDefault();
		profileError = '';
		profileSuccess = '';
		if (!fbUser) return;
		profileSaving = true;
		try {
			auth.user = await updateMe(profileBody(profileForm));
			await updateProfile(fbUser, {
				displayName: auth.user.name,
				photoURL: photoURL || null
			});
			profileSuccess = 'Zapisano';
		} catch (err: unknown) {
			profileError = (err as { code?: string }).code ? fbError(err) : errorMessage(err);
		} finally {
			profileSaving = false;
		}
	}

	async function handlePasswordSave(e: Event) {
		e.preventDefault();
		pwError = '';
		pwSuccess = '';
		if (!fbUser?.email) return;
		if (pwForm.next !== pwForm.confirm) {
			pwError = 'Hasła nie są takie same';
			return;
		}
		pwSaving = true;
		try {
			const cred = EmailAuthProvider.credential(fbUser.email, pwForm.current);
			await reauthenticateWithCredential(fbUser, cred);
			await updatePassword(fbUser, pwForm.next);
			pwForm = { current: '', next: '', confirm: '' };
			pwSuccess = 'Hasło zmienione';
		} catch (err: unknown) {
			pwError = fbError(err);
		} finally {
			pwSaving = false;
		}
	}
</script>

<div class="mx-auto max-w-2xl">
	<h1 class="mb-6 text-2xl font-bold text-[var(--navy)]">Profil</h1>

	<div class="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
		<h2 class="mb-4 font-semibold text-[var(--navy)]">Dane konta</h2>

		{#if profileError}
			<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{profileError}</div>
		{/if}
		{#if profileSuccess}
			<div class="mb-4 rounded-lg bg-[var(--ocean-tint)] text-[var(--ocean)] p-3 text-sm">
				{profileSuccess}
			</div>
		{/if}

		<form onsubmit={handleProfileSave} class="space-y-4">
			<div>
				<label class="mb-1 block text-sm font-medium text-gray-700" for="email">Email</label>
				<input
					id="email"
					type="text"
					value={auth.user?.email ?? fbUser?.email ?? ''}
					disabled
					class="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-500"
				/>
			</div>
			<ProfileFields bind:form={profileForm} />
			<div>
				<label class="mb-1 block text-sm font-medium text-gray-700" for="photoURL"
					>Adres zdjęcia (URL)</label
				>
				<input
					id="photoURL"
					type="url"
					bind:value={photoURL}
					placeholder="https://..."
					class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
				/>
			</div>
			{#if photoURL}
				<img
					src={photoURL}
					alt="Podgląd zdjęcia"
					class="h-20 w-20 rounded-full object-cover"
				/>
			{/if}
			<button
				type="submit"
				disabled={profileSaving}
				class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-white transition-colors hover:bg-[var(--ocean-dark)] disabled:opacity-50"
			>
				{profileSaving ? 'Zapisywanie...' : 'Zapisz'}
			</button>
		</form>
	</div>

	<div class="mt-6">
		<PersonClaim />
	</div>

	{#if hasPasswordProvider}
		<div class="mt-6 rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
			<h2 class="mb-4 font-semibold text-[var(--navy)]">Zmień hasło</h2>

			{#if pwError}
				<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{pwError}</div>
			{/if}
			{#if pwSuccess}
				<div class="mb-4 rounded-lg bg-[var(--ocean-tint)] text-[var(--ocean)] p-3 text-sm">{pwSuccess}</div>
			{/if}

			<form onsubmit={handlePasswordSave} class="space-y-4">
				<div>
					<label class="mb-1 block text-sm font-medium text-gray-700" for="curPw"
						>Obecne hasło</label
					>
					<input
						id="curPw"
						type="password"
						bind:value={pwForm.current}
						required
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
					/>
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium text-gray-700" for="newPw"
						>Nowe hasło</label
					>
					<input
						id="newPw"
						type="password"
						bind:value={pwForm.next}
						required
						minlength="8"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
					/>
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium text-gray-700" for="confirmPw"
						>Powtórz nowe hasło</label
					>
					<input
						id="confirmPw"
						type="password"
						bind:value={pwForm.confirm}
						required
						minlength="8"
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
					/>
				</div>
				<button
					type="submit"
					disabled={pwSaving}
					class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-white transition-colors hover:bg-[var(--ocean-dark)] disabled:opacity-50"
				>
					{pwSaving ? 'Zapisywanie...' : 'Zmień hasło'}
				</button>
			</form>
		</div>
	{/if}
</div>
