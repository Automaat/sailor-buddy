import '@testing-library/jest-dom/vitest';
import { vi } from 'vitest';

// $env/dynamic/public is a SvelteKit virtual module with no runtime value
// under vitest; stub it so modules importing it (api client, firebase) load.
vi.mock('$env/dynamic/public', () => ({ env: {} }));
