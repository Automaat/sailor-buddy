import { listCrew } from '#lib/api/routes.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async () => ({ people: (await listCrew()) ?? [] });
