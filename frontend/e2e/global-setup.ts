import { clearFirebaseUsers } from './helpers';

/**
 * Empties the Auth emulator once per run. A retried test group must keep the
 * accounts its first attempt created: the backend refuses to link a
 * re-registered email to an existing user, so a new Firebase UID for the
 * suite admin would get 401 on every request.
 */
export default async function globalSetup(): Promise<void> {
	await clearFirebaseUsers();
}
