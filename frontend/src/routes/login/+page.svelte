<script lang="ts">
	import { api } from '$lib/api/client';
	import { auth } from '$lib/stores/auth';
	import { goto } from '$app/navigation';
	import type { AuthResponse } from '$lib/api/types';

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
			const endpoint = isRegister ? '/auth/register' : '/auth/login';
			const body = isRegister ? { email, password, name } : { email, password };
			const data = await api.post<AuthResponse>(endpoint, body);
			auth.login(data.user, data.access_token, data.refresh_token);
			goto('/');
		} catch (err) {
			error = err instanceof Error ? err.message : 'Something went wrong';
		} finally {
			loading = false;
		}
	}
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
