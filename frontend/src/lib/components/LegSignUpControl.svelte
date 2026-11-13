<script lang="ts">
	import { signUpForTrip, updateTripSignUpNote, withdrawTripSignUp } from '#lib/api/routes.js';
	import type { LegSignUp } from '#lib/api/types.js';
	import { statusLabels } from '#lib/enrollment.js';

	interface Props {
		signUp: LegSignUp;
		legName: string;
		cancelled?: boolean;
		onchange: (next: LegSignUp) => void;
	}

	let { signUp, legName, cancelled = false, onchange }: Props = $props();

	let busy = $state(false);
	let error = $state('');
	let editingNote = $state(false);
	let noteDraft = $state('');

	const enrollment = $derived(signUp.enrollment);
	const cruiseLevel = $derived(enrollment?.cruise_id != null);

	const statusColors: Record<string, string> = {
		pending: 'bg-yellow-100 text-yellow-800',
		accepted: 'bg-green-100 text-green-800',
		rejected: 'bg-red-100 text-red-800',
		waitlisted: 'bg-purple-100 text-purple-800'
	};

	async function run(action: () => Promise<void>, failure: string) {
		busy = true;
		error = '';
		try {
			await action();
		} catch (err) {
			error = err instanceof Error && err.message ? err.message : failure;
		} finally {
			busy = false;
		}
	}

	function signUpNow() {
		return run(async () => {
			const res = await signUpForTrip(signUp.trip_id);
			onchange({ ...signUp, enrollment: res.enrollment });
		}, 'Nie udało się zapisać');
	}

	function withdraw() {
		return run(async () => {
			await withdrawTripSignUp(signUp.trip_id);
			editingNote = false;
			onchange({ ...signUp, enrollment: undefined });
		}, 'Nie udało się wycofać zapisu');
	}

	function startNote() {
		noteDraft = enrollment?.note ?? '';
		editingNote = true;
	}

	function saveNote(e: Event) {
		e.preventDefault();
		return run(async () => {
			const updated = await updateTripSignUpNote(signUp.trip_id, noteDraft);
			editingNote = false;
			onchange({ ...signUp, enrollment: updated });
		}, 'Nie udało się zapisać notatki');
	}
</script>

<div class="flex flex-col items-end gap-1 text-sm" data-testid="leg-sign-up">
	{#if enrollment}
		<div class="flex flex-wrap items-center justify-end gap-2">
			<span
				class="rounded-full px-2 py-0.5 text-xs font-medium {statusColors[enrollment.status] ?? 'bg-gray-100'}"
				data-testid="sign-up-status"
			>
				{statusLabels[enrollment.status] ?? enrollment.status}
			</span>
			{#if cruiseLevel}
				<span class="text-xs text-[var(--text-muted)]" data-testid="cruise-level">zapis na wydarzenie</span>
			{/if}
			{#if enrollment.status === 'pending' && !cruiseLevel}
				<button
					type="button"
					onclick={withdraw}
					disabled={busy}
					class="rounded px-2 py-1 text-xs text-red-600 hover:bg-red-50 disabled:opacity-50"
					aria-label={`Wycofaj zapis: ${legName}`}
				>
					Wycofaj
				</button>
			{/if}
			{#if !editingNote && !cruiseLevel}
				<button
					type="button"
					onclick={startNote}
					disabled={busy}
					class="rounded px-2 py-1 text-xs text-[var(--ocean)] hover:bg-blue-50 disabled:opacity-50"
				>
					{enrollment.note ? 'Edytuj notatkę' : 'Dodaj notatkę'}
				</button>
			{/if}
		</div>
		{#if enrollment.note && !editingNote}
			<p class="max-w-xs text-right text-xs text-[var(--text-muted)]" data-testid="sign-up-note">{enrollment.note}</p>
		{/if}
		{#if editingNote}
			<form onsubmit={saveNote} class="flex w-full max-w-xs flex-col items-end gap-1">
				<label for={`note-${signUp.trip_id}`} class="sr-only">Notatka dla organizatora</label>
				<textarea
					id={`note-${signUp.trip_id}`}
					bind:value={noteDraft}
					rows="2"
					maxlength="2000"
					placeholder="Wiadomość dla organizatora..."
					class="w-full rounded-lg border px-2 py-1 text-xs"
				></textarea>
				<div class="flex gap-1">
					<button type="button" onclick={() => (editingNote = false)} class="rounded px-2 py-1 text-xs hover:bg-gray-100">
						Anuluj
					</button>
					<button
						type="submit"
						disabled={busy}
						class="rounded bg-[var(--ocean)] px-2 py-1 text-xs text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
					>
						Zapisz notatkę
					</button>
				</div>
			</form>
		{/if}
	{:else if cancelled}
		<span class="text-xs text-[var(--text-muted)]">Etap odwołany</span>
	{:else if signUp.open}
		<button
			type="button"
			onclick={signUpNow}
			disabled={busy}
			class="rounded-lg bg-[var(--ocean)] px-4 py-1.5 text-sm font-medium text-white hover:bg-[var(--ocean-dark)] disabled:opacity-50"
			aria-label={`Zapisz się: ${legName}`}
		>
			{busy ? 'Zapisywanie...' : 'Zapisz się'}
		</button>
	{:else}
		<span class="text-xs text-[var(--text-muted)]">Zapisy zamknięte</span>
	{/if}
	{#if error}
		<p role="alert" class="text-xs text-red-600">{error}</p>
	{/if}
</div>
