<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { goto } from '$app/navigation';
	import {
		signInWithEmailAndPassword,
		createUserWithEmailAndPassword,
		signInWithPopup,
		GoogleAuthProvider,
		updateProfile
	} from 'firebase/auth';
	import { firebaseAuth } from '$lib/firebase';

	const googleProvider = new GoogleAuthProvider();

	let isRegister = $state(false);
	let email = $state('');
	let password = $state('');
	let name = $state('');
	let error = $state('');
	let loading = $state(false);

	async function handleSubmit(e: Event) {
		e.preventDefault();
		error = '';
		loading = true;

		try {
			if (isRegister) {
				const cred = await createUserWithEmailAndPassword(firebaseAuth, email, password);
				await updateProfile(cred.user, { displayName: name });
			} else {
				await signInWithEmailAndPassword(firebaseAuth, email, password);
			}
			goto('/');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Something went wrong';
		} finally {
			loading = false;
		}
	}

	async function handleGoogle() {
		error = '';
		loading = true;
		try {
			await signInWithPopup(firebaseAuth, googleProvider);
			goto('/');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Something went wrong';
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		if (auth.isAuthenticated) {
			goto('/');
		}
	});
</script>

<div class="flex min-h-screen items-center justify-center bg-[var(--navy)]">
	<div class="w-full max-w-md rounded-2xl bg-white p-8 shadow-xl">
		<div class="mb-8 text-center">
			<span class="text-5xl">⚓</span>
			<h1 class="mt-4 text-2xl font-bold text-[var(--navy)]">Sailor Buddy</h1>
			<p class="mt-1 text-[var(--text-muted)]">
				{isRegister ? 'Create your account' : 'Welcome back, Captain'}
			</p>
		</div>

		{#if error}
			<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
		{/if}

		<form onsubmit={handleSubmit} class="space-y-4">
			{#if isRegister}
				<div>
					<label for="name" class="mb-1 block text-sm font-medium">Name</label>
					<input
						id="name"
						type="text"
						bind:value={name}
						required
						class="w-full rounded-lg border border-gray-300 px-3 py-2 focus:border-[var(--ocean)] focus:outline-none focus:ring-1 focus:ring-[var(--ocean)]"
					/>
				</div>
			{/if}
			<div>
				<label for="email" class="mb-1 block text-sm font-medium">Email</label>
				<input
					id="email"
					type="email"
					bind:value={email}
					required
					class="w-full rounded-lg border border-gray-300 px-3 py-2 focus:border-[var(--ocean)] focus:outline-none focus:ring-1 focus:ring-[var(--ocean)]"
				/>
			</div>
			<div>
				<label for="password" class="mb-1 block text-sm font-medium">Password</label>
				<input
					id="password"
					type="password"
					bind:value={password}
					required
					minlength="8"
					class="w-full rounded-lg border border-gray-300 px-3 py-2 focus:border-[var(--ocean)] focus:outline-none focus:ring-1 focus:ring-[var(--ocean)]"
				/>
			</div>
			<button
				type="submit"
				disabled={loading}
				class="w-full rounded-lg bg-[var(--ocean)] px-4 py-2.5 font-medium text-white transition-colors hover:bg-[var(--ocean-dark)] disabled:opacity-50"
			>
				{loading ? '...' : isRegister ? 'Register' : 'Login'}
			</button>
		</form>

		<div class="relative my-6">
			<div class="absolute inset-0 flex items-center">
				<div class="w-full border-t border-gray-300"></div>
			</div>
			<div class="relative flex justify-center text-sm">
				<span class="bg-white px-2 text-[var(--text-muted)]">or</span>
			</div>
		</div>

		<button
			onclick={handleGoogle}
			disabled={loading}
			class="flex w-full items-center justify-center gap-3 rounded-lg border border-gray-300 bg-white px-4 py-2.5 font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-50"
		>
			<svg class="h-5 w-5" viewBox="0 0 24 24">
				<path
					d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"
					fill="#4285F4"
				/>
				<path
					d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
					fill="#34A853"
				/>
				<path
					d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z"
					fill="#FBBC05"
				/>
				<path
					d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
					fill="#EA4335"
				/>
			</svg>
			Continue with Google
		</button>

		<p class="mt-6 text-center text-sm text-[var(--text-muted)]">
			{isRegister ? 'Already have an account?' : "Don't have an account?"}
			<button
				onclick={() => {
					isRegister = !isRegister;
					error = '';
				}}
				class="font-medium text-[var(--ocean)] hover:underline"
			>
				{isRegister ? 'Login' : 'Register'}
			</button>
		</p>
	</div>
</div>
