<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import Download from '@lucide/svelte/icons/download';
	import CheckCircle2 from '@lucide/svelte/icons/check-circle-2';

	let fileInput = $state<HTMLInputElement | null>(null);
	let status = $state<'idle' | 'uploading' | 'preview' | 'confirming' | 'done' | 'error'>('idle');
	let error = $state('');
	let preview = $state<any>(null);

	function handleFileChange(e: Event) {
		const input = e.target as HTMLInputElement;
		const file = input.files?.[0];
		if (file) upload(file);
	}

	async function upload(file: File) {
		status = 'uploading';
		error = '';

		try {
			const token = await auth.getIdToken();
			if (!token) throw new Error('Not authenticated');
			const formData = new FormData();
			formData.append('file', file);

			const res = await fetch('/api/import/xlsx', {
				method: 'POST',
				headers: {
					Authorization: `Bearer ${token}`
				},
				body: formData
			});

			if (!res.ok) {
				const body = await res.json().catch(() => ({}));
				throw new Error(body.error || 'Przesyłanie nie powiodło się');
			}

			preview = await res.json();
			status = 'preview';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Przesyłanie nie powiodło się';
			status = 'error';
		}
	}

	async function handleConfirm() {
		status = 'confirming';
		try {
			const token = await auth.getIdToken();
			if (!token) throw new Error('Not authenticated');
			const res = await fetch('/api/import/confirm', {
				method: 'POST',
				headers: {
					'Content-Type': 'application/json',
					Authorization: `Bearer ${token}`
				},
				body: JSON.stringify(preview)
			});

			if (!res.ok) throw new Error('Import nie powiódł się');
			status = 'done';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Import nie powiódł się';
			status = 'error';
		}
	}
</script>

<input bind:this={fileInput} type="file" accept="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,.xlsx" onchange={handleFileChange} class="hidden" />

<div class="mx-auto max-w-2xl">
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Import z XLSX</h1>

	{#if status === 'idle' || status === 'error'}
		<div class="rounded-2xl bg-white p-8 shadow-sm">
			{#if error}
				<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
			{/if}
			<div class="text-center">
				<Download class="mx-auto mb-4 h-14 w-14 text-[var(--text-muted)]" />
				<p class="mb-4 text-[var(--text-muted)]">
					Prześlij arkusz żeglarski (XLSX), aby zaimportować rejsy, szkolenia i dane załogi.
				</p>
				<button
					onclick={() => fileInput?.click()}
					class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)]"
				>
					Wybierz plik i prześlij
				</button>
			</div>
		</div>
	{:else if status === 'uploading'}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<p class="text-lg text-[var(--text-muted)]">Przetwarzanie arkusza...</p>
		</div>
	{:else if status === 'preview' && preview}
		<div class="rounded-2xl bg-white p-6 shadow-sm">
			<h2 class="mb-4 font-semibold text-[var(--navy)]">Podgląd importu</h2>
			<p class="mb-2 text-sm text-[var(--text-muted)]">
				Znaleziono {preview.voyages?.length ?? 0} rejsów, {preview.trainings?.length ?? 0} szkoleń
			</p>
			<div class="mb-4 max-h-64 overflow-auto rounded-lg bg-gray-50 p-4 text-xs">
				<pre>{JSON.stringify(preview, null, 2)}</pre>
			</div>
			<div class="flex gap-3">
				<button onclick={handleConfirm} class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)]">
					Potwierdź import
				</button>
				<button onclick={() => { status = 'idle'; preview = null; }} class="rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50">
					Anuluj
				</button>
			</div>
		</div>
	{:else if status === 'confirming'}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<p class="text-lg text-[var(--text-muted)]">Importowanie danych...</p>
		</div>
	{:else if status === 'done'}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<CheckCircle2 class="mx-auto h-14 w-14 text-green-500" />
			<p class="mt-4 text-lg font-semibold text-[var(--navy)]">Import zakończony!</p>
			<a href="/voyages" class="mt-2 inline-block text-[var(--ocean)] hover:underline">Zobacz rejsy</a>
		</div>
	{/if}
</div>
