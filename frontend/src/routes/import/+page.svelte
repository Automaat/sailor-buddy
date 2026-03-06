<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';

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
				throw new Error(body.error || 'Upload failed');
			}

			preview = await res.json();
			status = 'preview';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Upload failed';
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

			if (!res.ok) throw new Error('Import failed');
			status = 'done';
		} catch (err) {
			error = err instanceof Error ? err.message : 'Import failed';
			status = 'error';
		}
	}
</script>

<input bind:this={fileInput} type="file" accept="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,.xlsx" onchange={handleFileChange} class="hidden" />

<div class="mx-auto max-w-2xl">
	<h1 class="mb-6 text-3xl font-bold text-[var(--navy)]">Import from XLSX</h1>

	{#if status === 'idle' || status === 'error'}
		<div class="rounded-2xl bg-white p-8 shadow-sm">
			{#if error}
				<div class="mb-4 rounded-lg bg-red-50 p-3 text-sm text-red-600">{error}</div>
			{/if}
			<div class="text-center">
				<p class="mb-4 text-5xl">📥</p>
				<p class="mb-4 text-[var(--text-muted)]">
					Upload your sailing spreadsheet (XLSX) to import cruises, trainings, and crew data.
				</p>
				<button
					onclick={() => fileInput?.click()}
					class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)]"
				>
					Choose File & Upload
				</button>
			</div>
		</div>
	{:else if status === 'uploading'}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<p class="text-lg text-[var(--text-muted)]">Parsing spreadsheet...</p>
		</div>
	{:else if status === 'preview' && preview}
		<div class="rounded-2xl bg-white p-6 shadow-sm">
			<h2 class="mb-4 font-semibold text-[var(--navy)]">Import Preview</h2>
			<p class="mb-2 text-sm text-[var(--text-muted)]">
				Found {preview.cruises?.length ?? 0} cruises, {preview.trainings?.length ?? 0} trainings
			</p>
			<div class="mb-4 max-h-64 overflow-auto rounded-lg bg-gray-50 p-4 text-xs">
				<pre>{JSON.stringify(preview, null, 2)}</pre>
			</div>
			<div class="flex gap-3">
				<button onclick={handleConfirm} class="rounded-lg bg-[var(--ocean)] px-6 py-2 font-medium text-white hover:bg-[var(--ocean-dark)]">
					Confirm Import
				</button>
				<button onclick={() => { status = 'idle'; preview = null; }} class="rounded-lg border px-6 py-2 text-[var(--text-muted)] hover:bg-gray-50">
					Cancel
				</button>
			</div>
		</div>
	{:else if status === 'confirming'}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<p class="text-lg text-[var(--text-muted)]">Importing data...</p>
		</div>
	{:else if status === 'done'}
		<div class="rounded-2xl bg-white py-16 text-center shadow-sm">
			<p class="text-5xl">✅</p>
			<p class="mt-4 text-lg font-semibold text-[var(--navy)]">Import Complete!</p>
			<a href="/cruises" class="mt-2 inline-block text-[var(--ocean)] hover:underline">View your cruises</a>
		</div>
	{/if}
</div>
