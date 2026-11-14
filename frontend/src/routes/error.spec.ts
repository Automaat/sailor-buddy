import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/svelte';

const page = vi.hoisted(() => ({
	status: 404,
	error: null as App.Error | null
}));
vi.mock('$app/state', () => ({ page }));

import ErrorPage from './+error.svelte';

beforeEach(() => {
	page.status = 404;
	page.error = null;
});

describe('root error page', () => {
	it('shows the not-found copy a load function threw', () => {
		page.error = {
			status: 404,
			message: 'To wydarzenie nie istnieje lub nie masz do niego dostępu.',
			title: 'Nie znaleziono wydarzenia',
			backHref: '/cruises',
			backLabel: 'Wszystkie rejsy'
		};
		render(ErrorPage);

		expect(screen.getByRole('heading', { name: 'Nie znaleziono wydarzenia' })).toBeInTheDocument();
		expect(screen.getByText('To wydarzenie nie istnieje lub nie masz do niego dostępu.')).toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'Wszystkie rejsy' })).toHaveAttribute('href', '/cruises');
	});

	it('falls back to the generic not-found copy for an unknown route', () => {
		page.error = { status: 404, message: 'Not Found' };
		render(ErrorPage);

		expect(screen.getByRole('heading', { name: 'Nie znaleziono' })).toBeInTheDocument();
		expect(screen.queryByText('Not Found')).not.toBeInTheDocument();
		expect(screen.getByText(/Nie znaleźliśmy tego, czego szukasz/)).toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'Wróć na start' })).toHaveAttribute('href', '/');
	});

	it('shows a generic failure for any other status', () => {
		page.status = 500;
		page.error = { status: 500, message: 'Internal Error' };
		render(ErrorPage);

		expect(screen.getByRole('heading', { name: 'Nie udało się wczytać strony' })).toBeInTheDocument();
		expect(screen.queryByText('Internal Error')).not.toBeInTheDocument();
	});
});
