import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import PriceCoverage from './PriceCoverage.svelte';

describe('PriceCoverage', () => {
	it('lists what is and is not included', () => {
		render(PriceCoverage, { included: ['jacht', 'skipper'], notIncluded: ['dojazd'] });
		expect(screen.getByText('W cenie')).toBeInTheDocument();
		expect(screen.getByTestId('price-included').querySelectorAll('li')).toHaveLength(2);
		expect(screen.getByText('Poza ceną')).toBeInTheDocument();
		expect(screen.getByTestId('price-not-included')).toHaveTextContent('dojazd');
	});

	it('shows only the list that has entries', () => {
		render(PriceCoverage, { included: null, notIncluded: ['dojazd'] });
		expect(screen.queryByText('W cenie')).not.toBeInTheDocument();
		expect(screen.getByTestId('price-not-included')).toHaveTextContent('dojazd');
	});

	it('renders nothing when both lists are empty', () => {
		const { container } = render(PriceCoverage, { included: [], notIncluded: null });
		expect(container.querySelector('section')).toBeNull();
	});
});
