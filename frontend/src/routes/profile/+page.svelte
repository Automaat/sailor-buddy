<script lang="ts">
	import {
		updateProfile,
		updatePassword,
		reauthenticateWithCredential,
		EmailAuthProvider
	} from 'firebase/auth';
	import { auth } from '$lib/stores/auth.svelte';
	import { updateMe } from '$lib/api/routes';

	const fbUser = auth.firebaseUser;

	type PatentType = 'zeglarz_jachtowy' | 'jachtowy_sternik_morski' | 'kapitan_jachtowy';

	const PATENT_OPTIONS: { value: '' | PatentType; label: string }[] = [
		{ value: '', label: '— brak —' },
		{ value: 'zeglarz_jachtowy', label: 'Żeglarz Jachtowy' },
		{ value: 'jachtowy_sternik_morski', label: 'Jachtowy Sternik Morski' },
		{ value: 'kapitan_jachtowy', label: 'Kapitan Jachtowy' }
	];

	let profileForm = $state({
		displayName: fbUser?.displayName ?? '',
		photoURL: fbUser?.photoURL ?? '',
		patentType: '' as '' | PatentType,
		patentNumber: ''
	});
	let profileSaving = $state(false);
	let profileError = $state('');
	let profileSuccess = $state('');

	// Patent fields come from the DB user (/auth/me), which the layout fetches
	// asynchronously. Seed the form once it arrives, before the user edits it.
	let patentLoaded = $state(false);
	$effect(() => {
		if (!patentLoaded && auth.user) {
			profileForm.patentType = (auth.user.patent_type as PatentType) ?? '';
			profileForm.patentNumber = auth.user.patent_number ?? '';
			patentLoaded = true;
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
			await updateProfile(fbUser, {
				displayName: profileForm.displayName,
				photoURL: profileForm.photoURL || null
			});
			// Force a fresh ID token so the next request carries the updated
			// `name` claim; the auth middleware upsert then syncs the DB.
			await fbUser.getIdToken(true);
			auth.user = await updateMe({
				patent_type: profileForm.patentType || undefined,
				patent_number: profileForm.patentNumber || undefined
			});
			profileSuccess = 'Zapisano';
		} catch (err: unknown) {
			profileError = fbError(err);
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
			<div class="mb-4 rounded-lg bg-green-50 p-3 text-sm text-green-600">
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
			<div>
				<label class="mb-1 block text-sm font-medium text-gray-700" for="displayName"
					>Imię i nazwisko *</label
				>
				<input
					id="displayName"
					type="text"
					bind:value={profileForm.displayName}
					required
					class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
				/>
			</div>
			<div>
				<label class="mb-1 block text-sm font-medium text-gray-700" for="photoURL"
					>Adres zdjęcia (URL)</label
				>
				<input
					id="photoURL"
					type="url"
					bind:value={profileForm.photoURL}
					placeholder="https://..."
					class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
				/>
			</div>
			{#if profileForm.photoURL}
				<img
					src={profileForm.photoURL}
					alt="Podgląd zdjęcia"
					class="h-20 w-20 rounded-full object-cover"
				/>
			{/if}
			<div class="grid grid-cols-2 gap-4">
				<div>
					<label class="mb-1 block text-sm font-medium text-gray-700" for="patentType"
						>Patent żeglarski</label
					>
					<select
						id="patentType"
						bind:value={profileForm.patentType}
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
					>
						{#each PATENT_OPTIONS as opt}
							<option value={opt.value}>{opt.label}</option>
						{/each}
					</select>
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium text-gray-700" for="patentNumber"
						>Numer patentu</label
					>
					<input
						id="patentNumber"
						type="text"
						bind:value={profileForm.patentNumber}
						disabled={!profileForm.patentType}
						class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm disabled:bg-gray-50 disabled:text-gray-400"
					/>
				</div>
			</div>
			<button
				type="submit"
				disabled={profileSaving}
				class="rounded-lg bg-[var(--ocean)] px-4 py-2 text-white transition-colors hover:bg-[var(--ocean-dark)] disabled:opacity-50"
			>
				{profileSaving ? 'Zapisywanie...' : 'Zapisz'}
			</button>
		</form>
	</div>

	{#if hasPasswordProvider}
		<div class="mt-6 rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
			<h2 class="mb-4 font-semibold text-[var(--navy)]">Zmień hasło</h2>

			{#if pwError}
				<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{pwError}</div>
			{/if}
			{#if pwSuccess}
				<div class="mb-4 rounded-lg bg-green-50 p-3 text-sm text-green-600">{pwSuccess}</div>
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
